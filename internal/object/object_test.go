package object

import "testing"

func TestObjectTypes(t *testing.T) {
	tests := []struct {
		name string
		obj  Object
		want ObjectType
	}{
		{"Integer", NewInteger(1), INTEGER_OBJ},
		{"Float", &Float{Value: 1.5}, FLOAT_OBJ},
		{"String", &String{Value: "x"}, STRING_OBJ},
		{"Boolean", TRUE, BOOLEAN_OBJ},
		{"Null", NULL, NULL_OBJ},
		{"List", NewList(nil), LIST_OBJ},
		{"Map", NewMap(), MAP_OBJ},
		{"Set", NewSet(), SET_OBJ},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.obj.Type(); got != tt.want {
				t.Errorf("%s.Type() = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
