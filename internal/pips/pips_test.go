package pips_test

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/pips"
	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/sip"
)

func TestNew(t *testing.T) {
	t.Parallel()

	type test struct {
		name string
		path string
		want pips.PIP
	}
	for _, tt := range []test{
		{
			name: "Returns an NRAA SIP PIP",
			path: "/path/to/SIP_20240606_NRAA_2024_001",
			want: pips.PIP{
				Path:         "/path/to/SIP_20240606_NRAA_2024_001",
				ManifestPath: "/path/to/SIP_20240606_NRAA_2024_001/objects/SIP_20240606_NRAA_2024_001/header/metadata.xml",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := pips.New(tt.path)
			assert.DeepEqual(t, p, tt.want)
		})
	}
}

func TestNewFromSIP(t *testing.T) {
	t.Parallel()

	s := sip.SIP{
		Path: "/path/to/SIP_20240606_NRAA_2024_001",
	}
	assert.DeepEqual(t, pips.NewFromSIP(s), pips.PIP{
		Path:         "/path/to/SIP_20240606_NRAA_2024_001",
		ManifestPath: "/path/to/SIP_20240606_NRAA_2024_001/objects/SIP_20240606_NRAA_2024_001/header/metadata.xml",
	})
}

func TestName(t *testing.T) {
	t.Parallel()

	p := pips.New("/path/to/SIP_20240606_NRAA_2024_001")
	assert.Equal(t, p.Name(), "SIP_20240606_NRAA_2024_001")
}

func TestConvertSIPPath(t *testing.T) {
	t.Parallel()

	p := pips.New("/path/to/SIP_20240606_NRAA_2024_001")
	assert.Equal(t,
		p.ConvertSIPPath("header/metadata.xml"),
		"objects/SIP_20240606_NRAA_2024_001/header/metadata.xml",
	)
	assert.Equal(t,
		p.ConvertSIPPath("content/f000001/d000001.txt"),
		"objects/SIP_20240606_NRAA_2024_001/content/f000001/d000001.txt",
	)
	assert.Equal(t, p.ConvertSIPPath("header/metadata.xsd"), "")
	assert.Equal(t, p.ConvertSIPPath("header/other.xml"), "")
}
