package httpapi

import (
	"net/http"
	"strconv"
)

const (
	defaultPageSize = 12
	maxPageSize     = 50
)

type pageRequest struct {
	Page     int
	PageSize int
}

func (p pageRequest) Offset() int { return (p.Page - 1) * p.PageSize }

// parsePage reads ?page=&pageSize=. Bad values fall back to the defaults and pageSize is capped.
func parsePage(r *http.Request) pageRequest {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}
	size, err := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if err != nil || size < 1 {
		size = defaultPageSize
	}
	return pageRequest{Page: page, PageSize: min(size, maxPageSize)}
}

type pageResponse[T any] struct {
	Items    []T `json:"items"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
	Total    int `json:"total"`
}

func newPage[T any](req pageRequest, items []T, total int) pageResponse[T] {
	if items == nil {
		items = []T{}
	}
	return pageResponse[T]{Items: items, Page: req.Page, PageSize: req.PageSize, Total: total}
}
