package diskutil

import "testing"

// TestEscapeFstabPath는 공백·탭·역슬래시가 fstab 필드를 나누지 않도록 검사한다.
func TestEscapeFstabPath(t *testing.T) {
	actual := escapeFstabPath("/data/a b\tback\\slash")
	expected := `/data/a\040b\011back\134slash`

	if actual != expected {
		t.Fatalf("경로 이스케이프 불일치: %q != %q", actual, expected)
	}
}

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
