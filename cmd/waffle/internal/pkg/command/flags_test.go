package command

import (
	"reflect"
	"strings"
	"testing"
)

func TestTakeValues(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		values []string
		rest   []string
	}{
		{name: "separate value", args: []string{"check", "--container", "a.go"}, values: []string{"a.go"}, rest: []string{"check"}},
		{name: "joined value", args: []string{"--container=a.go", "check"}, values: []string{"a.go"}, rest: []string{"check"}},
		{name: "repeated in both forms", args: []string{"--container", "a.go", "check", "--container=b.go"}, values: []string{"a.go", "b.go"}, rest: []string{"check"}},
		{name: "absent", args: []string{"check"}, rest: []string{"check"}},
		{name: "a longer flag is another flag", args: []string{"--containers=a.go"}, rest: []string{"--containers=a.go"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			values, rest, err := TakeValues(c.args, "--container", "a path")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(values, c.values) {
				t.Errorf("values = %v, want %v", values, c.values)
			}
			if !reflect.DeepEqual(rest, c.rest) {
				t.Errorf("rest = %v, want %v", rest, c.rest)
			}
		})
	}
}

func TestTakeValuesRefusesAFlagWithoutAValue(t *testing.T) {
	for _, args := range [][]string{{"check", "--grant"}, {"--grant="}} {
		_, _, err := TakeValues(args, "--grant", "an area's name")
		if err == nil || err.Error() != "--grant takes an area's name" {
			t.Errorf("%v: err = %v", args, err)
		}
	}
}

func TestTakeBool(t *testing.T) {
	set, rest := TakeBool([]string{"--batch", "user", "--batch", "order_paid"}, "--batch")
	if !set || !reflect.DeepEqual(rest, []string{"user", "order_paid"}) {
		t.Errorf("set = %v, rest = %v", set, rest)
	}
	set, rest = TakeBool([]string{"user", "--batched"}, "--batch")
	if set || !reflect.DeepEqual(rest, []string{"user", "--batched"}) {
		t.Errorf("set = %v, rest = %v", set, rest)
	}
}

func TestTakeDriver(t *testing.T) {
	cases := []struct {
		name string
		args []string
		id   string
		rest []string
		fail string
	}{
		{name: "separate value", args: []string{"--driver", "amqp", "user", "order_paid"}, id: "amqp", rest: []string{"user", "order_paid"}},
		{name: "joined value", args: []string{"--driver=amqp", "user", "order_paid"}, id: "amqp", rest: []string{"user", "order_paid"}},
		{name: "after the operands", args: []string{"user", "order_paid", "--driver", "amqp"}, id: "amqp", rest: []string{"user", "order_paid"}},
		{name: "no value", args: []string{"user", "order_paid", "--driver"}, fail: "broker module's name"},
		{name: "twice", args: []string{"--driver", "amqp", "--driver=nats"}, fail: "2 times"},
		// The module path is built from it, so it may not carry a separator of its own.
		{name: "a path", args: []string{"--driver", "../../service/amqp"}, fail: "lowercase Go identifier"},
		// Absent is not this function's refusal to make: Operands speaks first.
		{name: "absent", args: []string{"user", "order_paid"}, rest: []string{"user", "order_paid"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, rest, err := TakeDriver(c.args)
			if c.fail != "" {
				if err == nil || !strings.Contains(err.Error(), c.fail) {
					t.Fatalf("TakeDriver(%v) = %q, %v; want an error with %q", c.args, id, err, c.fail)
				}
				return
			}
			if err != nil {
				t.Fatalf("TakeDriver(%v): %v", c.args, err)
			}
			if id != c.id {
				t.Errorf("driver = %q, want %q", id, c.id)
			}
			if !reflect.DeepEqual(rest, c.rest) {
				t.Errorf("operands = %v, want %v", rest, c.rest)
			}
		})
	}
}
