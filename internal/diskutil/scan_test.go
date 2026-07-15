package diskutil

import "testing"

func TestIsDangerousMountpoint(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/", want: true},
		{path: "/etc", want: true},
		{path: "/etc/", want: true},
		{path: "/data", want: false},
		{path: "/data/", want: false},
		{path: "/storage", want: false},
		{path: "/data/hdd_1tb", want: false},
	}

	for _, tt := range tests {
		if got := IsDangerousMountpoint(tt.path); got != tt.want {
			t.Fatalf("IsDangerousMountpoint(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
