/*
Sniperkit-Bot
- Status: analyzed
*/

package types

import (
	"encoding/json"
	"testing"
	"time"

	"we.com/jiabiao/common/yaml"
)

func TestDeployConfig_Key(t *testing.T) {
	tests := []struct {
		name string
		dc   *DeployConfig
		want UUID
	}{
		{
			name: "test1",
			dc: &DeployConfig{
				Type:          ProjectType("java"),
				Name:          DeployName("crm-server"),
				NumOfInstance: 3,
				ServiceType:   ServiceService,
				Stage:         QA,
				Image:         MustParseImageName("java/crm-server"),
				DeployDir:     "/usr/local/java",
				Values: map[string]interface{}{
					"abc": "efg",
					"133": 123,
				},
				DeployPolicy: Inplace,

				// these fields used to select which hosts can start this project
				Selector: map[string]string{
					"":     "java",
					"host": "!=java-uat",
					"net":  "outer",
				},
				RestartPolicy: &RestartPolicy{Type: OneTime},
				UpdatePolicy: &UpdateOption{
					Policy:  RollingUpdate,
					Step:    30 * time.Second,
					Timeout: 5 * time.Minute,
				},
			},
			want: "java/crm-server",
		},
	}
	for _, tt := range tests {
		d, err := json.Marshal(tt.dc)
		if err != nil {
			t.Errorf("unexpected erro: %v", err)
		}
		bs, err := yaml.ToYAML([]byte(d))
		if err != nil {
			t.Errorf("toyaml: %v", err)
		}

		t.Errorf("%s", bs)
	}
}
