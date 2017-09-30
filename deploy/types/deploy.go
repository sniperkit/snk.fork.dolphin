package types

import (
	"context"

	"we.com/dolphin/deploy/conf/template"
	"we.com/dolphin/types"
)

// DeployOptions options config when deploy
type DeployOptions struct {
	Image   types.Image
	DstPath string
	Force   bool
	New     bool
	Name    string
	Confd   *template.Config
	Inplace bool
}

// DeployManager  interface to manager deploys
type DeployManager interface {
	Deploy(image string, dst string, opts ...DeployOptions) error
}

// FileDeliverer  handle file delivery only
type FileDeliverer interface {
	// Delivery  delivery file with curry images
	Delivery(ctx context.Context, image string, dst string, force bool) error
	Recovery(dst string) error
}
