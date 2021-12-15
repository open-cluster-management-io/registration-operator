package manifests

import "embed"

//go:embed cluster-manager
var ClusterManagerManifestFiles embed.FS

//go:embed klusterlet
//go:embed klusterlet/detached
//go:embed klusterletkube111
var KlusterletManifestFiles embed.FS

// var Klusterlet111ManifestFiles embed.FS
