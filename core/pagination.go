package core

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-fuego/fuego"
)

const (
	// DefaultPaginationLimit is the default page size when none is specified.
	DefaultPaginationLimit = 50
	// MaxPaginationLimit is the maximum permissible page size.
	MaxPaginationLimit = 1000
)

// PaginationInput defines standardized pagination request query parameters.
type PaginationInput struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// PaginationOutput defines standardized pagination metadata for response payloads.
type PaginationOutput struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Count  int `json:"count"`
	Total  int `json:"total,omitempty"`
}

// PaginatedResponse defines a generic standardized response payload for paginated endpoints.
type PaginatedResponse[T any] struct {
	Data []T `json:"data"`
	PaginationOutput
}

// ParsePagination extracts and normalizes limit and offset query parameters from an HTTP request.
func ParsePagination(request *http.Request) PaginationInput {
	return ParsePaginationWithDefaults(request, DefaultPaginationLimit, MaxPaginationLimit)
}

// ParsePaginationWithDefaults extracts and normalizes limit and offset with custom boundary values.
func ParsePaginationWithDefaults(request *http.Request, defaultLimit, maxLimit int) PaginationInput {
	limit := defaultLimit
	if rawLimit := request.URL.Query().Get("limit"); rawLimit != "" {
		if parsedLimit, err := strconv.Atoi(rawLimit); err == nil && parsedLimit > 0 {
			if maxLimit > 0 && parsedLimit > maxLimit {
				limit = maxLimit
			} else {
				limit = parsedLimit
			}
		}
	}

	offset := 0
	if rawOffset := request.URL.Query().Get("offset"); rawOffset != "" {
		if parsedOffset, err := strconv.Atoi(rawOffset); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	return PaginationInput{
		Limit:  limit,
		Offset: offset,
	}
}

// RoutePagination attaches standardized limit and offset query parameters to an OpenAPI route.
func RoutePagination(defaultLimit, maxLimit int) RouteOption {
	return func(baseRoute *fuego.BaseRoute) {
		fuego.OptionQueryInt("limit", fmt.Sprintf("Maximum number of items to return (default %d, maximum %d)", defaultLimit, maxLimit), fuego.ParamDefault(defaultLimit))(baseRoute)
		fuego.OptionQueryInt("offset", "Number of items to skip before returning results (default 0)", fuego.ParamDefault(0))(baseRoute)
	}
}

// RouteDefaultPagination attaches standard pagination using DefaultPaginationLimit and MaxPaginationLimit.
func RouteDefaultPagination() RouteOption {
	return RoutePagination(DefaultPaginationLimit, MaxPaginationLimit)
}
