package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadParityRegionManifest(t *testing.T) {
	regions, err := loadParityRegionManifest(filepath.Join("testdata", "regions-v1.json"))
	if err != nil {
		t.Fatalf("loadParityRegionManifest: %v", err)
	}
	if len(regions) != 1 || len(regions[15]) != 1 {
		t.Fatalf("regions = %#v", regions)
	}
	region := regions[15][0]
	if region.Name != "canonical-structure" || region.Bounds.MinX != 0 || region.Bounds.MaxY != 16 {
		t.Fatalf("region = %#v", region)
	}

	copyForPair := cloneParityRegions(regions[15])
	copyForPair[0].Name = "mutated"
	if regions[15][0].Name != "canonical-structure" {
		t.Fatalf("cloneParityRegions mutated manifest state: %#v", regions[15])
	}
}

func TestLoadParityRegionManifestRejectsInvalidPolicy(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "version",
			data: `{"version":2,"frames":[]}`,
			want: "version = 2, want 1",
		},
		{
			name: "duplicate frame",
			data: `{"version":1,"frames":[{"frame_index":2,"regions":[]},{"frame_index":2,"regions":[]}]}`,
			want: "duplicate frame_index 2",
		},
		{
			name: "negative frame",
			data: `{"version":1,"frames":[{"frame_index":-1,"regions":[]}]}`,
			want: "frame_index must be non-negative",
		},
		{
			name: "empty name",
			data: `{"version":1,"frames":[{"frame_index":1,"regions":[{"name":" ","bounds":{"min_x":0,"min_y":0,"max_x":10,"max_y":10}}]}]}`,
			want: "name must not be empty",
		},
		{
			name: "duplicate region",
			data: `{"version":1,"frames":[{"frame_index":1,"regions":[{"name":"same","bounds":{"min_x":0,"min_y":0,"max_x":10,"max_y":10}},{"name":"same","bounds":{"min_x":10,"min_y":10,"max_x":20,"max_y":20}}]}]}`,
			want: "duplicate region name",
		},
		{
			name: "invalid bounds",
			data: `{"version":1,"frames":[{"frame_index":1,"regions":[{"name":"bad","bounds":{"min_x":5,"min_y":0,"max_x":5,"max_y":10}}]}]}`,
			want: "bounds must define a positive non-negative rectangle",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "regions.json")
			if err := os.WriteFile(path, []byte(test.data), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := loadParityRegionManifest(path)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestLoadParityRegionManifestEmptyPath(t *testing.T) {
	regions, err := loadParityRegionManifest("   ")
	if err != nil || regions != nil {
		t.Fatalf("regions = %#v, err = %v", regions, err)
	}
}
