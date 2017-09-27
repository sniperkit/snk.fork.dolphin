package types

import (
	"testing"
)

func Test_resUnit_String(t *testing.T) {
	tests := []struct {
		name string
		ru   resUnit
		want string
	}{
		{
			name: "k",
			ru:   resUnit(1000 * 1000 * 5),
			want: "5M",
		},
		{
			name: "m",
			ru:   resUnit(1000 * 1000 * 512 * 5),
			want: "2G",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ru.String(); got != tt.want {
				t.Errorf("resUnit.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeployResource_String(t *testing.T) {
	dr := DeployResource{
		Memory:    1234567890,
		CPU:       1423452289,
		DiskSpace: 12345678912345,
	}

	a := dr.String()
	b := (&dr).String()

	if a != b {
		t.Errorf("a, b: %v,%v", a, b)
	}
}

func TestParseResoureValue(t *testing.T) {
	type args struct {
		res string
	}
	tests := []struct {
		name    string
		args    args
		want    uint64
		wantErr bool
	}{
		{
			name: "0.5m",
			args: args{
				res: "0.5m",
			},
			want:    500000,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseResoureValue(tt.args.res)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseResoureValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseResoureValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
