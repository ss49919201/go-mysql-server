package parsedate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseDate(t *testing.T) {
	setupTimezone(t)

	tests := [...]struct {
		name     string
		date     string
		format   string
		expected string
	}{
		{"simple", "Jan 3, 2000", "%b %e, %Y", "2000-01-03 00:00:00 -0600 CST"},
		{"simple_with_spaces", "Nov  03 ,   2000", "%b %e, %Y", "2000-11-03 00:00:00 -0600 CST"},
		{"simple_with_spaces_2", "Dec  15 ,   2000", "%b %e, %Y", "2000-12-15 00:00:00 -0600 CST"},
		{"reverse", "2023/Feb/ 1", "%Y/%b/%e", "2023-02-01 00:00:00 -0600 CST"},
		{"reverse_with_spaces", " 2023 /Apr/ 01  ", "%Y/%b/%e", "2023-04-01 00:00:00 -0500 CDT"},
		{"weekday", "Thu, Aug 5, 2021", "%a, %b %e, %Y", "2021-08-05 00:00:00 -0500 CDT"},

		{"with_time", "Sep 3, 22:23:00 2000", "%b %e, %H:%i:%s %Y", "2000-09-03 22:23:00 -0500 CDT"},
		{"with_pm", "May 3, 10:23:00 PM 2000", "%b %e, %H:%i:%s %p %Y", "2000-05-03 22:23:00 -0500 CDT"},
		{"lowercase_pm", "Jul 3, 10:23:00 pm 2000", "%b %e, %H:%i:%s %p %Y", "2000-07-03 22:23:00 -0500 CDT"},
		{"with_am", "Mar 3, 10:23:00 am 2000", "%b %e, %H:%i:%s %p %Y", "2000-03-03 10:23:00 -0600 CST"},

		{"month_number", "1 3, 10:23:00 pm 2000", "%c %e, %H:%i:%s %p %Y", "2000-01-03 22:23:00 -0600 CST"},

		{"day_with_suffix", "Jun 3rd, 10:23:00 pm 2000", "%b %D, %H:%i:%s %p %Y", "2000-06-03 22:23:00 -0500 CDT"},
		{"day_with_suffix_2", "Oct 21st, 10:23:00 pm 2000", "%b %D, %H:%i:%s %p %Y", "2000-10-21 22:23:00 -0500 CDT"},
		{"with_timestamp", "01/02/2003, 12:13:14", "%c/%d/%Y, %T", "2003-01-02 12:13:14 -0600 CST"},

		{"month_number", "03: 3, 20", "%m: %e, %y", "2020-03-03 00:00:00 -0600 CST"},
		{"month_name", "march: 3, 20", "%M: %e, %y", "2020-03-03 00:00:00 -0600 CST"},
		{"two_digit_date", "january: 3, 20", "%M: %e, %y", "2020-01-03 00:00:00 -0600 CST"},
		{"two_digit_date_2000", "september: 3, 70", "%M: %e, %y", "1970-09-03 00:00:00 -0500 CDT"},
		{"two_digit_date_1900", "may: 3, 69", "%M: %e, %y", "2069-05-03 00:00:00 -0500 CDT"},

		{"microseconds", "01/02/99 314", "%m/%e/%y %f", "1999-01-02 00:00:00.000314 -0600 CST"},
		{"hour_number", "01/02/99 5:14", "%m/%e/%y %h:%i", "1999-01-02 05:14:00 -0600 CST"},
		{"hour_number_2", "01/02/99 5:14", "%m/%e/%y %I:%i", "1999-01-02 05:14:00 -0600 CST"},

		{"timestamp", "01/02/99 05:14:12 PM", "%m/%e/%y %r", "1999-01-02 17:14:12 -0600 CST"},
		{"date_with_seconds", "01/02/99 57", "%m/%e/%y %S", "1999-01-02 00:00:57 -0600 CST"},

		{"date_by_year_offset", "100 20", "%j %y", "2020-04-09 00:00:00 -0500 CDT"},
		{"date_by_year_offset_singledigit_year", "100 5", "%j %y", "2005-04-10 00:00:00 -0500 CDT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := ParseDateWithFormat(tt.date, tt.format)
			require.NoError(t, err)
			require.Equal(t, tt.expected, actual.String())
		})
	}
}

func setupTimezone(t *testing.T) {
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	old := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = old })
}

func TestAmbiguous(t *testing.T) {
	tests := [...]struct {
		name          string
		date          string
		format        string
		expectedError string
	}{
		{"simple", "Jan 3", "%b %e", "year is ambiguous"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDateWithFormat(tt.date, tt.format)
			require.Error(t, err)
			require.Equal(t, tt.expectedError, err.Error())
		})
	}
}

func TestParseErr(t *testing.T) {
	tests := [...]struct {
		name          string
		date          string
		format        string
		expectedError interface{}
	}{
		{"simple", "a", "b", ParseLiteralErr{Literal: 'b', Tokens: "a"}},
		{"bad_numeral", "abc", "%e", ParseSpecifierErr{Specifier: 'e', Tokens: "abc"}},
		{"bad_month", "1 Jen, 2000", "%e %b, %Y", ParseSpecifierErr{Specifier: 'b', Tokens: "jen, 2000"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDateWithFormat(tt.date, tt.format)
			require.Error(t, err)
			require.Equal(t, tt.expectedError.(error).Error(), err.Error())
			require.ErrorAs(t, err, &tt.expectedError)
		})
	}
}
