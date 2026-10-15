package version

import "testing"

func TestHuman(t *testing.T) {
	tests := []struct{ version, commit, want string }{
		{"dev", "", "dev"},
		{"v0.1.0", "", "v0.1.0"},
		{"v0.1.0", "a1b2c3d", "v0.1.0 (a1b2c3d)"},
		{"dev", "a1b2c3d", "dev (a1b2c3d)"},
		// "none" is what the build stamps when there is no git to ask.
		{"dev", "none", "dev"},
		{"", "", "dev"},
	}
	for _, tt := range tests {
		if got := Human(tt.version, tt.commit); got != tt.want {
			t.Errorf("Human(%q, %q) = %q, want %q", tt.version, tt.commit, got, tt.want)
		}
	}
}

func TestLong(t *testing.T) {
	tests := []struct{ version, commit, date, want string }{
		{"v0.1.0", "a1b2c3d", "2026-10-15T10:00:00Z", "v0.1.0 (a1b2c3d), built 2026-10-15T10:00:00Z"},
		// A build from source shows no date.
		{"dev", "none", "unknown", "dev"},
		{"", "", "", "dev"},
	}
	for _, tt := range tests {
		if got := Long(tt.version, tt.commit, tt.date); got != tt.want {
			t.Errorf("Long(%q, %q, %q) = %q, want %q", tt.version, tt.commit, tt.date, got, tt.want)
		}
	}
}

func TestFullUsesStampedValues(t *testing.T) {
	saved := [3]string{Version, Commit, Date}
	t.Cleanup(func() { Version, Commit, Date = saved[0], saved[1], saved[2] })

	Version, Commit, Date = "v0.1.0", "a1b2c3d", "2026-10-15T10:00:00Z"
	if got, want := Full(), "v0.1.0 (a1b2c3d), built 2026-10-15T10:00:00Z"; got != want {
		t.Errorf("Full() = %q, want %q", got, want)
	}
}
