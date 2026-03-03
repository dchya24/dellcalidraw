package room

import (
	"encoding/json"
	"fmt"
	"regexp"
)

const (
	MaxElementSize     = 10240 // 10KB max per element
	MaxElementsPerRoom = 5000  // Maximum elements per room
	MaxStringLength    = 1000  // Max string length for text fields
)

var validElementTypes = regexp.MustCompile(`^(rectangle|ellipse|arrow|line|freedraw|text|image)$`)

// ValidationError represents an element validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidateElement validates an element's data
func ValidateElement(element Element) []ValidationError {
	var errors []ValidationError

	// Validate element type
	if element.Type == "" {
		errors = append(errors, ValidationError{
			Field:   "type",
			Message: "Element type is required",
		})
	} else if !validElementTypes.MatchString(element.Type) {
		errors = append(errors, ValidationError{
			Field:   "type",
			Message: fmt.Sprintf("Invalid element type: %s", element.Type),
		})
	}

	// Validate element ID
	if element.ID == "" {
		errors = append(errors, ValidationError{
			Field:   "id",
			Message: "Element ID is required",
		})
	}

	// Validate coordinates
	if element.X < 0 || element.X > 100000 {
		errors = append(errors, ValidationError{
			Field:   "x",
			Message: "X coordinate out of valid range",
		})
	}

	if element.Y < 0 || element.Y > 100000 {
		errors = append(errors, ValidationError{
			Field:   "y",
			Message: "Y coordinate out of valid range",
		})
	}

	// Validate dimensions (allow 0 for newly created elements)
	if element.Width < 0 || element.Width > 100000 {
		errors = append(errors, ValidationError{
			Field:   "width",
			Message: "Width out of valid range",
		})
	}

	if element.Height < 0 || element.Height > 100000 {
		errors = append(errors, ValidationError{
			Field:   "height",
			Message: "Height out of valid range",
		})
	}

	// Validate data size
	if element.Data != nil {
		dataBytes, err := json.Marshal(element.Data)
		if err != nil {
			errors = append(errors, ValidationError{
				Field:   "data",
				Message: "Invalid data format",
			})
		} else if len(dataBytes) > MaxElementSize {
			errors = append(errors, ValidationError{
				Field:   "data",
				Message: fmt.Sprintf("Element data exceeds maximum size of %d bytes", MaxElementSize),
			})
		}
	}

	// Validate text elements
	if element.Type == "text" {
		if text, ok := element.Data["text"].(string); ok {
			if len(text) > MaxStringLength {
				errors = append(errors, ValidationError{
					Field:   "data.text",
					Message: "Text exceeds maximum length",
				})
			}
		}
	}

	return errors
}

// ValidateElementsBatch validates multiple elements
func ValidateElementsBatch(elements []Element) []ValidationError {
	var allErrors []ValidationError
	for i, elem := range elements {
		elemErrors := ValidateElement(elem)
		for _, err := range elemErrors {
			allErrors = append(allErrors, ValidationError{
				Field:   fmt.Sprintf("elements[%d].%s", i, err.Field),
				Message: err.Message,
			})
		}
	}
	return allErrors
}

// ValidateElementCount checks if adding elements would exceed room limit
func ValidateElementCount(currentCount, addingCount int) error {
	newCount := currentCount + addingCount
	if newCount > MaxElementsPerRoom {
		return fmt.Errorf("cannot add elements: room would exceed maximum of %d elements", MaxElementsPerRoom)
	}
	return nil
}
