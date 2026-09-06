package caps

import (
	"reflect"
	"testing"
)

func TestIntersectIsNotUnion(t *testing.T) {
	got := Intersect(
		[]string{ScreenRead, InputControl, AudioRead},
		[]string{ScreenRead, InputControl},
		[]string{ScreenRead},
	)
	want := []string{ScreenRead}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestSubset(t *testing.T) {
	if !Subset([]string{ScreenRead}, []string{ScreenRead, InputControl}) {
		t.Fatal("expected subset")
	}
	if Subset([]string{AudioRead}, []string{ScreenRead}) {
		t.Fatal("audio is not requested")
	}
}
