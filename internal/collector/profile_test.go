package collector_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phperf/phperf/internal/collector"
)

func TestDecodeRaw(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    collector.RawProfile
		wantErr bool
	}{
		{
			name: "profil linéaire",
			data: `{
				"main()": {"ct": 1, "wt": 1000, "cpu": 900, "mu": 50000, "pmu": 45000},
				"main()==>strlen": {"ct": 2, "wt": 10, "cpu": 8, "mu": 64, "pmu": 64}
			}`,
			want: collector.RawProfile{
				"main()":          {CT: 1, WT: 1000, CPU: 900, MU: 50000, PMU: 45000},
				"main()==>strlen": {CT: 2, WT: 10, CPU: 8, MU: 64, PMU: 64},
			},
			wantErr: false,
		},
		{
			name:    "JSON invalide",
			data:    `{main():}`,
			wantErr: true,
		},
		{
			name:    "JSON vide",
			data:    `{}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := collector.DecodeRaw([]byte(tt.data))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDecodeRaw_InvalidJSONMentionsSource(t *testing.T) {
	_, err := collector.DecodeRaw([]byte(`{main():}`))
	require.ErrorContains(t, err, "collector")
}
