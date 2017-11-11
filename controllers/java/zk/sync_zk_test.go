package zk

import (
	"testing"
)

func Test_manager_ReloadData(t *testing.T) {
	type args struct {
		env string
	}
	tests := []struct {
		name    string
		m       *manager
		args    args
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.m.ReloadData(tt.args.env); (err != nil) != tt.wantErr {
				t.Errorf("manager.ReloadData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
