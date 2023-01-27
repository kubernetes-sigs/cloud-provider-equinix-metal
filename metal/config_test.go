package metal

import (
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func Test_getMetalConfig(t *testing.T) {
	type args struct {
		providerConfig io.Reader
	}

	testKey, testProject := "test", "test"
	defaultConfig := Config{
		// not default, but required and set in tests
		AuthToken: testKey,
		ProjectID: testProject,
		// defaults defined in getMetalConfig
		LocalASN:                     DefaultLocalASN,
		AnnotationLocalASN:           DefaultAnnotationNodeASN,
		AnnotationPeerASN:            DefaultAnnotationPeerASN,
		AnnotationPeerIP:             DefaultAnnotationPeerIP,
		AnnotationSrcIP:              DefaultAnnotationSrcIP,
		AnnotationBGPPass:            DefaultAnnotationBGPPass,
		AnnotationNetworkIPv4Private: DefaultAnnotationNetworkIPv4Private,
		AnnotationEIPMetro:           DefaultAnnotationEIPMetro,
		AnnotationEIPFacility:        DefaultAnnotationEIPFacility,
	}
	tests := []struct {
		name    string
		args    args
		want    Config
		wantErr bool
		env     map[string]string
	}{
		{
			name: "no config",
			args: args{
				providerConfig: nil,
			},
			want:    defaultConfig,
			wantErr: false,
			env: map[string]string{
				"METAL_API_KEY":    "test",
				"METAL_PROJECT_ID": "test",
			},
		},
		{
			name: "empty config file",
			args: args{
				providerConfig: strings.NewReader(""),
			},
			want:    Config{},
			wantErr: true,
		},
		{
			name: "empty json config file",
			args: args{
				providerConfig: strings.NewReader("{}"),
			},
			want:    Config{},
			wantErr: true, // environment variable "METAL_API_KEY" is required if not in config file
		},
		{
			name: "empty json config file with env",
			args: args{
				providerConfig: strings.NewReader("{}"),
			},
			want:    defaultConfig,
			wantErr: false,
			env: map[string]string{
				"METAL_API_KEY":    "test",
				"METAL_PROJECT_ID": "test",
			},
		},
		{
			name: "json config",
			args: args{
				providerConfig: strings.NewReader(`{
					"apiKey": "test",
					"projectId": "test"
				}`),
			},
			want:    defaultConfig,
			wantErr: false,
		},
		{
			name: "partial json config",
			args: args{
				providerConfig: strings.NewReader(`{
					"apiKey": "test",
				}`),
			},
			want:    Config{},
			wantErr: true,
		},
		{
			name: "invalid json",
			args: args{
				providerConfig: strings.NewReader(`{]`),
			},
			want:    Config{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO(displague) env is a global potentially polluting other tests. getMetalConfig should accept envs as an argument.
			for k, v := range tt.env {
				os.Setenv(k, v)
			}
			got, err := getMetalConfig(tt.args.providerConfig)
			for k := range tt.env {
				os.Unsetenv(k)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("getMetalConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getMetalConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}
