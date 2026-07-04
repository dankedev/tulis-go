package response

import (
	"time"

	"github.com/gofiber/fiber/v2"
)

type ResponseMeta struct {
	RequestID  string      `json:"request_id"`
	Timestamp  string      `json:"timestamp"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

type Pagination struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

type ErrorDetail struct {
	Field   string      `json:"field,omitempty"`
	Message string      `json:"message"`
	Code    string      `json:"code"`
	Value   interface{} `json:"value,omitempty"`
}

type SuccessResponse struct {
	Status  int         `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ErrorResponse struct {
	Status  int         `json:"status"`
	Message string      `json:"message"`
	Errors  interface{} `json:"errors,omitempty"`
}

type ErrorContent struct {
	Code    string        `json:"code"`
	Message string        `json:"message"`
	Details []ErrorDetail `json:"details,omitempty"`
	TraceID string        `json:"trace_id,omitempty"`
}

func Success(c *fiber.Ctx, data interface{}, message string) error {
	if message == "" {
		message = "Success"
	}

	return c.Status(fiber.StatusOK).JSON(SuccessResponse{
		Status:  fiber.StatusOK,
		Message: message,
		Data:    data,
	})
}

func Error(c *fiber.Ctx, code string, message string, errors interface{}) error {
	status := getHTTPCode(code)
	return c.Status(status).JSON(ErrorResponse{
		Status:  status,
		Message: message,
		Errors:  errors,
	})
}

// ErrorJSON returns a fiber.Map for use with c.JSON() - use this when you need to chain with .JSON()
func ErrorJSON(c *fiber.Ctx, code string, message string, details []ErrorDetail, traceID string) fiber.Map {
	if traceID == "" {
		traceID = c.Get("X-Request-ID")
	}

	return fiber.Map{
		"success": false,
		"error": fiber.Map{
			"code":     code,
			"message":  message,
			"details":  details,
			"trace_id": traceID,
		},
		"meta": fiber.Map{
			"request_id": traceID,
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		},
	}
}

func getHTTPCode(code string) int {
	switch code {
	case "BAD_REQUEST", "VALIDATION_ERROR":
		return fiber.StatusBadRequest
	case "UNAUTHORIZED":
		return fiber.StatusUnauthorized
	case "FORBIDDEN":
		return fiber.StatusForbidden
	case "NOT_FOUND":
		return fiber.StatusNotFound
	case "RATE_LIMITED":
		return fiber.StatusTooManyRequests
	case "SERVICE_UNAVAILABLE":
		return fiber.StatusServiceUnavailable
	default:
		return fiber.StatusInternalServerError
	}
}

func GlobalErrorHandler(c *fiber.Ctx, err error) error {
	message := err.Error()

	return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
		Status:  fiber.StatusInternalServerError,
		Message: message,
	})
}
