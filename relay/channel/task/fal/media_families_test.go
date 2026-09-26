package fal

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func mediaInfo(origin, upstream string) *relaycommon.RelayInfo {
	info := &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
		ChannelMeta:   &relaycommon.ChannelMeta{},
	}
	info.OriginModelName = origin
	info.UpstreamModelName = upstream
	return info
}

func TestResolveMediaRouteH3MaxTurboDefaults(t *testing.T) {
	req := relaycommon.TaskSubmitReq{Model: "h3-max-turbo", Prompt: "a test"}
	route, ok, err := resolveMediaRoute(mediaInfo("h3-max-turbo", ""), req)
	if !ok || err != nil {
		t.Fatalf("expected route, ok=%v err=%v", ok, err)
	}
	if route.Endpoint != "minimax/h3-max-turbo/text-to-video" {
		t.Fatalf("wrong endpoint %s", route.Endpoint)
	}
}

func TestResolveMediaRouteH3MaxTurboI2VWithAudioPin(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:  "h3-max-turbo",
		Prompt: "test",
		Image:  "https://example.com/first.png",
		Metadata: map[string]any{
			"reference_audios": []string{"https://example.com/voice.wav"},
		},
	}
	route, ok, err := resolveMediaRoute(mediaInfo("h3-max-turbo", ""), req)
	if !ok || err != nil {
		t.Fatalf("expected route, ok=%v err=%v", ok, err)
	}
	if !route.UseI2V || route.Endpoint != "minimax/h3-max-turbo/image-to-video" {
		t.Fatalf("expected i2v, got %+v", route)
	}
	body, err := buildMediaRequestBody(req, route)
	if err != nil {
		t.Fatal(err)
	}
	if body["image_url"] != "https://example.com/first.png" {
		t.Fatalf("missing image_url: %+v", body)
	}
	if body["target_audio_url"] != "https://example.com/voice.wav" {
		t.Fatalf("missing target_audio_url: %+v", body)
	}
}

func TestResolveMediaRouteH3MaxR2VAudioOnly(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:  "h3-max",
		Prompt: "test",
		Metadata: map[string]any{
			"reference_audios": []string{"https://example.com/voice.wav"},
		},
	}
	route, ok, err := resolveMediaRoute(mediaInfo("h3-max", ""), req)
	if !ok || err != nil {
		t.Fatalf("expected route, ok=%v err=%v", ok, err)
	}
	if !route.UseR2V || route.Endpoint != "minimax/h3-max/reference-to-video" {
		t.Fatalf("expected r2v, got %+v", route)
	}
	body, err := buildMediaRequestBody(req, route)
	if err != nil {
		t.Fatal(err)
	}
	if audios, ok := body["audio_urls"].([]string); !ok || len(audios) != 1 {
		t.Fatalf("missing audio_urls: %+v", body)
	}
}

func TestResolveMediaRouteH3MaxImagesPlusAudio(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:  "h3-max",
		Prompt: "test",
		Metadata: map[string]any{
			"reference_images": []string{"https://example.com/a.png", "https://example.com/b.png"},
			"reference_audios": []string{"https://example.com/voice.wav"},
		},
	}
	route, ok, err := resolveMediaRoute(mediaInfo("h3-max", ""), req)
	if !ok || err != nil {
		t.Fatalf("expected route, ok=%v err=%v", ok, err)
	}
	if !route.UseR2V {
		t.Fatalf("expected r2v, got %+v", route)
	}
	if route.Action != constant.TaskActionReferenceGenerate {
		t.Fatalf("wrong action %s", route.Action)
	}
}

func TestResolveMediaRouteTurboRejectsVideoRefs(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:  "h3-max-turbo",
		Prompt: "test",
		Metadata: map[string]any{
			"reference_videos": []string{"https://example.com/clip.mp4"},
		},
	}
	_, ok, err := resolveMediaRoute(mediaInfo("h3-max-turbo", ""), req)
	if !ok || err == nil {
		t.Fatalf("expected loud error, ok=%v err=%v", ok, err)
	}
	if !strings.Contains(err.Error(), "reference videos") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveMediaRouteKlingRejectsAudio(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:  "kling-3-pro",
		Prompt: "test",
		Metadata: map[string]any{
			"reference_audios": []string{"https://example.com/voice.wav"},
		},
	}
	_, ok, err := resolveMediaRoute(mediaInfo("kling-3-pro", ""), req)
	if !ok || err == nil {
		t.Fatalf("expected loud error, ok=%v err=%v", ok, err)
	}
}

func TestBuildMediaRequestBodyParams(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:  "h3-max",
		Prompt: "test",
		Image:  "https://example.com/first.png",
		Metadata: map[string]any{
			"last_frame_image":    "https://example.com/last.png",
			"resolution":          "768p",
			"duration":            8,
			"aspect_ratio":        "16:9",
			"seed":                42,
			"prompt_expansion_mode": "disabled",
		},
	}
	route, ok, err := resolveMediaRoute(mediaInfo("h3-max", ""), req)
	if !ok || err != nil {
		t.Fatalf("expected route, ok=%v err=%v", ok, err)
	}
	body, err := buildMediaRequestBody(req, route)
	if err != nil {
		t.Fatal(err)
	}
	if body["resolution"] != "768P" {
		t.Fatalf("resolution not uppercased: %v", body["resolution"])
	}
	if body["duration"] != 8 {
		t.Fatalf("duration missing: %+v", body)
	}
	if body["aspect_ratio"] != "16:9" {
		t.Fatalf("aspect_ratio missing: %+v", body)
	}
	if body["image_url"] == nil || body["end_image_url"] == nil {
		t.Fatalf("frames missing: %+v", body)
	}
	if body["seed"] != 42 {
		t.Fatalf("seed passthrough missing: %+v", body)
	}
}

func TestBuildMediaRequestBodyWan3AudioBool(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:  "wan-3",
		Prompt: "test",
		Metadata: map[string]any{
			"generate_audio": true,
			"duration":       5,
		},
	}
	route, ok, err := resolveMediaRoute(mediaInfo("wan-3", ""), req)
	if !ok || err != nil {
		t.Fatalf("expected route, ok=%v err=%v", ok, err)
	}
	body, err := buildMediaRequestBody(req, route)
	if err != nil {
		t.Fatal(err)
	}
	if body["audio"] != true {
		t.Fatalf("generate_audio not mapped to audio: %+v", body)
	}
	if _, exists := body["generate_audio"]; exists {
		t.Fatalf("generate_audio should not pass through for wan-3: %+v", body)
	}
}

func TestExplicitEndpointRouting(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:  "h3-max",
		Prompt: "test",
		Image:  "https://example.com/first.png",
	}
	info := mediaInfo("h3-max", "minimax/h3-max/image-to-video")
	route, ok, err := resolveMediaRoute(info, req)
	if !ok || err != nil {
		t.Fatalf("expected route, ok=%v err=%v", ok, err)
	}
	if route.Endpoint != "minimax/h3-max/image-to-video" || !route.UseI2V {
		t.Fatalf("explicit endpoint not honored: %+v", route)
	}
}

func TestMediaFamiliesInModelList(t *testing.T) {
	list := falMediaModels()
	for _, want := range []string{
		"h3-max-turbo", "h3-max", "kling-3-pro", "wan-3",
		"minimax/h3-max/reference-to-video",
	} {
		found := false
		for _, m := range list {
			if m == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("model %s missing from list", want)
		}
	}
}
