package types

import (
	"encoding/json"
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

	d, err := json.MarshalIndent(cfg, "", "\t")
	if err != nil {
		t.Error(err)
	}
	t.Logf("cfg: %v", string(d))
}
