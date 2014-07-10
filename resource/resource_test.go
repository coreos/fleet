package resource

import "reflect"
import "testing"

func TestSum(t *testing.T) {
	for i, tt := range []struct {
		in   []ResourceTuple
		want ResourceTuple
	}{
		{
			[]ResourceTuple{ResourceTuple{10, 24, 1024}},
			ResourceTuple{10, 24, 1024},
		},
		{
			[]ResourceTuple{ResourceTuple{10, 24, 1024}, ResourceTuple{10, 24, 1024}},
			ResourceTuple{20, 48, 2048},
		},
		{
			[]ResourceTuple{},
			ResourceTuple{0, 0, 0},
		},
	} {
		got := Sum(tt.in...)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("case %d: got %v, want %v", i, got, tt.want)
		}
	}
}

func TestSub(t *testing.T) {
	for i, tt := range []struct {
		r1   ResourceTuple
		r2   ResourceTuple
		want ResourceTuple
	}{
		{
			ResourceTuple{10, 24, 1024},
			ResourceTuple{10, 24, 1024},
			ResourceTuple{0, 0, 0},
		},
		{
			ResourceTuple{20, 48, 2048},
			ResourceTuple{15, 36, 2048},
			ResourceTuple{5, 12, 0},
		},
		{
			ResourceTuple{0, 0, 0},
			ResourceTuple{0, 0, 0},
			ResourceTuple{0, 0, 0},
		},
	} {
		got := Sub(tt.r1, tt.r2)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("case %d: got %v, want %v", i, got, tt.want)
		}
	}
}
