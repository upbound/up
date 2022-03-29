package xpkg

import (
	"archive/tar"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/pkg/errors"
	"github.com/spf13/afero"

	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
)

func TestXPExtractRun(t *testing.T) {
	errBoom := errors.New("boom")
	randLayer, _ := random.Layer(int64(1000), types.DockerLayer)
	randImg, _ := mutate.Append(empty.Image, mutate.Addendum{
		Layer: randLayer,
		Annotations: map[string]string{
			layerAnnotation: baseAnnotationValue,
		},
	})

	randImgDup, _ := mutate.Append(randImg, mutate.Addendum{
		Layer: randLayer,
		Annotations: map[string]string{
			layerAnnotation: baseAnnotationValue,
		},
	})

	streamCont := "somestreamofyaml"
	tarBuf := new(bytes.Buffer)
	tw := tar.NewWriter(tarBuf)
	hdr := &tar.Header{
		Name: xpkg.StreamFile,
		Mode: int64(xpkg.StreamFileMode),
		Size: int64(len(streamCont)),
	}
	_ = tw.WriteHeader(hdr)
	_, _ = io.Copy(tw, strings.NewReader(streamCont))
	_ = tw.Close()

	packLayer, _ := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		// NOTE(hasheddan): we must construct a new reader each time as we
		// ingest packImg in multiple tests below.
		return io.NopCloser(bytes.NewReader(tarBuf.Bytes())), nil
	})
	packImg, _ := mutate.AppendLayers(empty.Image, packLayer)
	cases := map[string]struct {
		reason string
		fs     afero.Fs
		img    image.Fetcher
		tag    string
		out    string
		want   error
	}{
		"ErrorInvalidTag": {
			reason: "Should return error if we fail to parse package name.",
			tag:    "++++",
			want:   errors.Wrap(errors.New("could not parse reference: ++++"), errInvalidTag),
		},
		"ErrorFetchPackage": {
			reason: "Should return error if we fail to fetch package.",
			tag:    "crossplane/provider-aws:v0.24.1",
			img:    image.NewMockFetcher(image.WithError(errBoom)),
			want:   errors.Wrap(errBoom, errFetchPackage),
		},
		"ErrorMultipleAnnotatedLayers": {
			reason: "Should return error if manifest contains multiple annotated layers.",
			tag:    "crossplane/provider-aws:v0.24.1",
			img:    image.NewMockFetcher(image.WithImage(randImgDup)),
			want:   errors.New(errMultipleAnnotatedLayers),
		},
		"ErrorFetchBadPackage": {
			reason: "Should return error if image with contents does not have package.yaml.",
			tag:    "crossplane/provider-aws:v0.24.1",
			img:    image.NewMockFetcher(image.WithImage(randImg)),
			want:   errors.Wrap(io.EOF, errOpenPackageStream),
		},
		"Success": {
			reason: "Should not return error if we successfully fetch package and extract contents.",
			tag:    "crossplane/provider-aws:v0.24.1",
			img:    image.NewMockFetcher(image.WithImage(packImg)),
			fs:     afero.NewMemMapFs(),
			out:    "out.gz",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := (&xpExtractCmd{
				fs:      tc.fs,
				img:     tc.img,
				Package: tc.tag,
				Output:  tc.out,
			}).Run()
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
