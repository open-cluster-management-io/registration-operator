package manifests

import "embed"

//go:embed cluster-manager
var ClusterManagerManifestFiles embed.FS

//go:embed cluster-manager-external-hub
var ClusterManagerExternalHubManifestFiles embed.FS

//go:embed cluster-manager-hosted
var ClusterManagerHostedManifestFiles embed.FS

//go:embed klusterlet
var KlusterletManifestFiles embed.FS

//go:embed klusterletkube111
var Klusterlet111ManifestFiles embed.FS
