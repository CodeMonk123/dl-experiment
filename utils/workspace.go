package utils

import "fmt"

const (
	podNamePrefix            = "dlkit-workspace"
	WorkspaceVolumeMountPath = "/workspace"
)

func GeneralWorkspacePVC(workspaceName string) string {
	return fmt.Sprintf("%s-%s", podNamePrefix, workspaceName)
}
