package schedule

import (
	"errors"
	"testing"
	"time"
)

func TestFiveFieldCronParserNext(t *testing.T) {
	t.Parallel()
	parser := FiveFieldCronParser{}
	utc := time.UTC
	after := time.Date(2026, time.July, 23, 10, 7, 42, 0, utc)

	got, err := parser.Next("*/15 9-17 * * 1-5", after, utc)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, time.July, 23, 10, 15, 0, 0, utc)
	if !got.Equal(want) {
		t.Fatalf("next = %s, want %s", got, want)
	}
}

func TestFiveFieldCronParserUsesTraditionalDOMOrDOWSemantics(t *testing.T) {
	t.Parallel()
	parser := FiveFieldCronParser{}
	// July 31, 2026 is a Friday. "day 1 OR Friday" therefore fires on it.
	after := time.Date(2026, time.July, 30, 23, 59, 0, 0, time.UTC)
	got, err := parser.Next("0 0 1 * 5", after, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, time.July, 31, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("next = %s, want %s", got, want)
	}
}

func TestFiveFieldCronParserAcceptsSevenAsSunday(t *testing.T) {
	t.Parallel()
	parser := FiveFieldCronParser{}
	after := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC) // Friday
	got, err := parser.Next("0 9 * * 7", after, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, time.July, 26, 9, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("next = %s, want %s", got, want)
	}
}

func TestFiveFieldCronParserSkipsNonexistentDSTTime(t *testing.T) {
	t.Parallel()
	location, err := time.LoadLocation("America/Toronto")
	if err != nil {
		t.Fatal(err)
	}
	parser := FiveFieldCronParser{}
	// Toronto jumps from 01:59 to 03:00 on this date.
	after := time.Date(2026, time.March, 8, 1, 59, 0, 0, location)
	got, err := parser.Next("30 2 * * *", after, location)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, time.March, 9, 2, 30, 0, 0, location)
	if !got.Equal(want) {
		t.Fatalf("next = %s, want %s", got, want)
	}
}

func TestFiveFieldCronParserRejectsInvalidExpressions(t *testing.T) {
	t.Parallel()
	parser := FiveFieldCronParser{}
	for _, expression := range []string{
		"* * * *",
		"60 * * * *",
		"*/0 * * * *",
		"* 9-2 * * *",
		"* * * * monday",
	} {
		_, err := parser.Next(expression, time.Now(), time.UTC)
		if !errors.Is(err, ErrInvalidCron) {
			t.Errorf("%q error = %v, want ErrInvalidCron", expression, err)
		}
	}
}
