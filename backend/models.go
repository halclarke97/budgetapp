package main

import "time"

type Expense struct {
	ID                 string    `json:"id"`
	Amount             float64   `json:"amount"`
	Category           string    `json:"category"`
	Note               string    `json:"note"`
	Date               time.Time `json:"date"`
	CreatedAt          time.Time `json:"created_at"`
	RecurringPatternID *string   `json:"recurring_pattern_id,omitempty"`
}

type RecurringPattern struct {
	ID          string     `json:"id"`
	Amount      float64    `json:"amount"`
	Category    string     `json:"category"`
	Note        string     `json:"note"`
	Frequency   string     `json:"frequency"`
	StartDate   time.Time  `json:"start_date"`
	NextRunDate time.Time  `json:"next_run_date"`
	EndDate     *time.Time `json:"end_date,omitempty"`
	Active      bool       `json:"active"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Category struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type CategoryTotal struct {
	Category string  `json:"category"`
	Total    float64 `json:"total"`
}

type DailyTotal struct {
	Date  string  `json:"date"`
	Total float64 `json:"total"`
}

type Stats struct {
	TotalExpenses int             `json:"total_expenses"`
	TotalAmount   float64         `json:"total_amount"`
	Period        string          `json:"period"`
	PeriodTotal   float64         `json:"period_total"`
	ByCategory    []CategoryTotal `json:"by_category"`
	Trend         []DailyTotal    `json:"trend"`
}
