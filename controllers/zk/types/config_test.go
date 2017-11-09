package types

import (
	"os"
	"testing"

	"we.com/jiabiao/common/yaml"
)

func TestConfig_Validate(t *testing.T) {
	reader, err := os.Open("./cfg.yml")
	if err != nil {
		t.Error(err)
	}

	decode := yaml.NewYAMLOrJSONDecoder(reader, 4)

	cfg := Config{}
	err = decode.Decode(&cfg)
	if err != nil {
		t.Error(err)
	}

	err = cfg.Validate()
	if err != nil {
		t.Error(err)
	}
}
