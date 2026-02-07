import { Link } from 'react-router-dom';

export function NotFoundView() {
  return (
    <section className="page">
      <article className="panel">
        <h1>Page not found</h1>
        <p>The page you requested does not exist.</p>
        <Link className="btn btn-primary" to="/dashboard">
          Go to Dashboard
        </Link>
      </article>
    </section>
  );
}
