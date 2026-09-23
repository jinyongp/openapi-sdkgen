package releaseconfig

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"go.yaml.in/yaml/v4"
)

type config struct {
	Version int `yaml:"version"`
	Builds  []struct {
		ID      string   `yaml:"id"`
		Main    string   `yaml:"main"`
		Binary  string   `yaml:"binary"`
		LDFlags []string `yaml:"ldflags"`
		GOOS    []string `yaml:"goos"`
		GOARCH  []string `yaml:"goarch"`
	} `yaml:"builds"`
	Archives []struct {
		ID           string   `yaml:"id"`
		IDs          []string `yaml:"ids"`
		NameTemplate string   `yaml:"name_template"`
		Formats      []string `yaml:"formats"`
		Overrides    []struct {
			GOOS    string   `yaml:"goos"`
			Formats []string `yaml:"formats"`
		} `yaml:"format_overrides"`
	} `yaml:"archives"`
	Checksum struct {
		NameTemplate string `yaml:"name_template"`
	} `yaml:"checksum"`
}

func TestReleaseConfigurationMatchesSupportedBinaryContract(t *testing.T) {
	path := filepath.Join("..", "..", ".goreleaser.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value config
	if err := yaml.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	if value.Version != 2 || len(value.Builds) != 1 {
		t.Fatalf("release config version/builds = %#v", value)
	}
	build := value.Builds[0]
	if build.ID != "openapi-sdkgen" || build.Main != "./cmd/openapi-sdkgen" || build.Binary != "openapi-sdkgen" {
		t.Fatalf("build = %#v", build)
	}
	if !slices.Equal(build.LDFlags, []string{"-s -w -X main.version={{ .Version }}"}) {
		t.Fatalf("build ldflags = %#v", build.LDFlags)
	}
	if !slices.Equal(build.GOOS, []string{"darwin", "linux", "windows"}) || !slices.Equal(build.GOARCH, []string{"amd64", "arm64"}) {
		t.Fatalf("platforms = %#v", build)
	}
	if len(value.Archives) != 2 {
		t.Fatalf("archives = %#v", value.Archives)
	}
	archive := value.Archives[0]
	if archive.ID != "default" ||
		!slices.Equal(archive.Formats, []string{"tar.gz"}) ||
		len(archive.Overrides) != 1 ||
		archive.Overrides[0].GOOS != "windows" ||
		!slices.Equal(archive.Overrides[0].Formats, []string{"zip"}) {
		t.Fatalf("default archive = %#v", archive)
	}
	npmBinary := value.Archives[1]
	if npmBinary.ID != "npm-binary" ||
		!slices.Equal(npmBinary.IDs, []string{"openapi-sdkgen"}) ||
		npmBinary.NameTemplate != "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}{{ .ArtifactExt }}" ||
		!slices.Equal(npmBinary.Formats, []string{"binary"}) ||
		len(npmBinary.Overrides) != 0 {
		t.Fatalf("npm binary archive = %#v", npmBinary)
	}
	if value.Checksum.NameTemplate != "checksums.txt" {
		t.Fatalf("checksum = %#v", value.Checksum)
	}
}
