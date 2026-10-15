// Package domain holds the entities, value types, errors and authorization
// rules shared by every service. It has no dependencies on adapters or
// transports.
package domain

import "strconv"

// ID is a 64-bit Snowflake: time-ordered, so sorting by ID sorts by creation
// time. It travels as a decimal string on the wire to stay safe in JavaScript.
type ID int64

func (id ID) String() string { return strconv.FormatInt(int64(id), 10) }

// ParseID parses a wire ID; field names the request field for the error.
func ParseID(s, field string) (ID, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return 0, Invalid(FieldViolation{Field: field, Reason: ReasonFieldInvalidFormat})
	}
	return ID(n), nil
}
