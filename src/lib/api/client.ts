export interface ApiErrorPayload {
  error?: {
    code?: string;
    message?: string;
    details?: unknown;
  };
}

export class ApiError extends Error {
  public readonly status: number;
  public readonly code: string;
  public readonly details?: unknown;

  constructor(args: { status: number; code: string; message: string; details?: unknown }) {
    super(args.message);
    this.name = 'ApiError';
    this.status = args.status;
    this.code = args.code;
    this.details = args.details;
  }
}

export function isApiError(error: unknown): error is ApiError {
  return error instanceof ApiError;
}

type QueryValue = string | number | boolean | null | undefined;

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  query?: Record<string, QueryValue>;
  body?: unknown;
  headers?: HeadersInit;
  signal?: AbortSignal;
}

export class ApiClient {
  constructor(private readonly baseUrl: string) {}

  public async get<TResponse>(path: string, options: Omit<RequestOptions, 'method' | 'body'> = {}): Promise<TResponse> {
    return this.request<TResponse>(path, { ...options, method: 'GET' });
  }

  public async post<TResponse>(path: string, body: unknown, options: Omit<RequestOptions, 'method' | 'body'> = {}): Promise<TResponse> {
    return this.request<TResponse>(path, { ...options, method: 'POST', body });
  }

  public async put<TResponse>(path: string, body: unknown, options: Omit<RequestOptions, 'method' | 'body'> = {}): Promise<TResponse> {
    return this.request<TResponse>(path, { ...options, method: 'PUT', body });
  }

  public async delete<TResponse>(path: string, options: Omit<RequestOptions, 'method' | 'body'> = {}): Promise<TResponse> {
    return this.request<TResponse>(path, { ...options, method: 'DELETE' });
  }

  public async request<TResponse>(path: string, options: RequestOptions = {}): Promise<TResponse> {
    const { method = 'GET', query, body, headers, signal } = options;

    const requestUrl = new URL(`${this.baseUrl}${path}`, window.location.origin);
    if (query) {
      for (const [key, value] of Object.entries(query)) {
        if (value !== undefined && value !== null && value !== '') {
          requestUrl.searchParams.append(key, String(value));
        }
      }
    }

    const response = await fetch(requestUrl.toString(), {
      method,
      headers: {
        'Content-Type': 'application/json',
        ...headers
      },
      body: body ? JSON.stringify(body) : undefined,
      signal
    });

    const contentType = response.headers.get('content-type') ?? '';
    const isJson = contentType.includes('application/json');
    const payload = isJson ? await response.json() : await response.text();

    if (!response.ok) {
      const typedPayload = payload as ApiErrorPayload;
      const apiCode = typedPayload?.error?.code;
      const apiMessage = typedPayload?.error?.message;

      throw new ApiError({
        status: response.status,
        code: apiCode ?? `HTTP_${response.status}`,
        message: apiMessage ?? `Request failed with status ${response.status}`,
        details: typedPayload?.error?.details
      });
    }

    return payload as TResponse;
  }
}

export const apiClient = new ApiClient(import.meta.env.VITE_API_BASE_URL ?? '/api/v1');
