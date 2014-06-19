package api

import (
	"testing"
)

func TestIsCollectionPathPass(t *testing.T) {
	tests := []struct {
		base string
		arg  string
	}{
		{"/v1", "/v1"},
		{"/v1/units", "/v1/units"},
		{"v1/units", "v1/units"},
	}

	for i, tt := range tests {
		if !isCollectionPath(tt.base, tt.arg) {
			t.Errorf("case %d: expected success with base=%s arg=%s", i, tt.base, tt.arg)
		}
	}
}

func TestIsCollectionPathFail(t *testing.T) {
	tests := []struct {
		base string
		arg  string
	}{
		{"/v1/units", "/v1/units/"},
		{"/v1/units", "/v1/units/item"},
		{"/v1/units", "/v1/units/item/"},
		{"/v1/units", "units"},
	}

	for i, tt := range tests {
		if isCollectionPath(tt.base, tt.arg) {
			t.Errorf("case %d: expected failure with base=%s arg=%s", i, tt.base, tt.arg)
		}
	}
}

func TestIsItemPathPass(t *testing.T) {
	tests := []struct {
		base string
		arg  string
		item string
	}{
		{"/", "/foo", "foo"},
		{"/v1/units/", "/v1/units/foo", "foo"},
		{"/v1/units/", "/v1/units/foo.service", "foo.service"},
	}

	for i, tt := range tests {
		item, ok := isItemPath(tt.base, tt.arg)
		if !ok {
			t.Errorf("case %d: expected success with base=%s arg=%s", i, tt.base, tt.arg)
		} else if item != tt.item {
			t.Errorf("case %d: expected item=%s, got %s", i, tt.item, item)
		}
	}
}

func TestIsItemPathFail(t *testing.T) {
	tests := []struct {
		base string
		arg  string
	}{
		{"/units", "/units"},
		{"/v1/units", "/v1/units"},
		{"/v1/units/", "/v1/units"},
		{"/v1/units/", "/v1/units/"},
		{"/v1/units", "/v1/units/foo/bar"},
		{"/v1/units/", "/v1/units/foo/bar"},
	}

	for i, tt := range tests {
		if _, ok := isItemPath(tt.base, tt.arg); ok {
			t.Errorf("case %d: expected failure with base=%s arg=%s", i, tt.base, tt.arg)
		}
	}
}
