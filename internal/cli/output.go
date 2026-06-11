package cli

import (
	"encoding/json"
	"fmt"
)

// Output formats data as either JSON or human-readable.
type Output struct {
	format string // "json" or "table"
}

// NewOutput creates a new output formatter.
func NewOutput(format string) *Output {
	return &Output{format: format}
}

// Print prints the given value in the configured format.
func (o *Output) Print(v interface{}) error {
	if o.format == "json" {
		bytes, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(bytes))
		return nil
	}
	// Table format handled by FormatTable
	return nil
}
