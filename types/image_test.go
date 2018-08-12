/*
Sniperkit-Bot
- Status: analyzed
*/

package types

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestParseImageName(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name    string
		args    args
		want    *Image
		wantErr bool
	}{
		{
			name: "test1",
			args: args{
				name: "java/crm-server:v1.1.1+buidl10",
			},
			want: &Image{
				Name:    "java/crm-server",
				Version: MustParseVersion("v1.1.1+buidl10"),
			},
			wantErr: false,
		},
		{
			name: "test2",
			args: args{
				name: "java/crm-server",
			},
			want: &Image{
				Name: "java/crm-server",
			},
			wantErr: false,
		},
		{
			name: "test3",
			args: args{
				name: "java/crm-server:v1.1.1",
			},
			want: &Image{
				Name:    "java/crm-server",
				Version: MustParseVersion("v1.1.1"),
			},
			wantErr: false,
		},
		{
			name: "test3",
			args: args{
				name: "java_crm/crm-server:v1.1.1",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseImageName(tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseImageName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseImageName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestImage_Validate(t *testing.T) {
	tests := []struct {
		name    string
		i       Image
		wantErr bool
	}{
		{
			name: "test1",
			i: Image{
				Name: "java/crm-sev",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.i.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Image.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestImage_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		want    *Image
		args    string
		wantErr bool
	}{
		{
			name: "test1",
			want: &Image{
				Name:    "java/crm",
				Version: MustParseVersion("v1.1.1"),
			},
			args:    "\"java/crm:v1.1.1\"",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &Image{}
			err := json.Unmarshal(([]byte)(tt.args), got)
			if (err != nil) != tt.wantErr {
				t.Errorf("Image.UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Image.UnmarshalJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}
