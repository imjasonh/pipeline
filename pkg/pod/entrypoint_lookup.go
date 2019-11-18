package pod

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	lru "github.com/hashicorp/golang-lru"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// resolveEntrypoints looks up container image ENTRYPOINTs for all steps that
// don't specify a Command.
//
// Images that are not specified by digest will be specified by digest after
// lookup in the resulting list of containers.
func resolveEntrypoints(cache EntrypointCache, namespace, serviceAccountName string, containers []corev1.Container) ([]corev1.Container, error) {
	var out []corev1.Container
	for _, s := range containers {
		if len(s.Command) != 0 {
			// Nothing to resolve.
			out = append(out, s)
			continue
		}

		ep, digest, err := cache.Get(s.Image, namespace, serviceAccountName)
		if err != nil {
			return nil, err
		}
		out = append(out, corev1.Container{
			Image:   digest.String(), // Specify image by digest, since we know it.
			Command: ep,
			// passthrough
			Args:         s.Args,
			Env:          s.Env,
			VolumeMounts: s.VolumeMounts,
			WorkingDir:   s.WorkingDir,
			Resources:    s.Resources,
		})
	}
	return out, nil
}

const cacheSize = 1024

type EntrypointCache interface {
	Get(imageName, namespace, serviceAccountName string) (cmd []string, d name.Digest, err error)
}

type entrypointCache struct {
	kubeclient kubernetes.Interface
	lru        *lru.Cache // cache of digest string -> image entrypoint []string
}

func NewEntrypointCache(kubeclient kubernetes.Interface) (EntrypointCache, error) {
	lru, err := lru.New(cacheSize)
	if err != nil {
		return nil, err
	}
	return &entrypointCache{
		kubeclient: kubeclient,
		lru:        lru,
	}, nil
}

func (e *entrypointCache) Get(imageName, namespace, serviceAccountName string) (cmd []string, d name.Digest, err error) {
	ref, err := name.ParseReference(imageName, name.WeakValidation)
	if err != nil {
		return nil, name.Digest{}, fmt.Errorf("Error parsing reference %q: %v", imageName, err)
	}

	// If image is specified by digest, check the local cache.
	if digest, ok := ref.(name.Digest); ok {
		if ep, ok := e.lru.Get(digest.String()); ok {
			return ep.([]string), digest, nil
		}
	}

	// If the image wasn't specified by digest, or if the entrypoint
	// wasn't found, we have to consult the remote registry, using
	// imagePullSecrets.
	kc, err := k8schain.New(e.kubeclient, k8schain.Options{
		Namespace:          namespace,
		ServiceAccountName: serviceAccountName,
	})
	if err != nil {
		return nil, name.Digest{}, fmt.Errorf("Error creating k8schain: %v", err)
	}
	img, err := remote.Image(ref, remote.WithAuthFromKeychain(kc))
	if err != nil {
		return nil, name.Digest{}, fmt.Errorf("Error getting image manifest: %v", err)
	}
	digest, err := img.Digest()
	if err != nil {
		return nil, name.Digest{}, fmt.Errorf("Error getting image digest: %v", err)
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, name.Digest{}, fmt.Errorf("Error getting image config: %v", err)
	}
	ep := cfg.Config.Entrypoint
	e.lru.Add(digest.String(), ep) // Populate the cache.

	d, err = name.NewDigest(imageName+"@"+digest.String(), name.WeakValidation)
	if err != nil {
		return nil, name.Digest{}, fmt.Errorf("Error constructing resulting digest: %v", err)
	}
	return ep, d, nil
}
