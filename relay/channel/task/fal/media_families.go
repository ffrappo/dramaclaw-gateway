package fal

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	basefal "github.com/QuantumNous/new-api/relay/channel/fal"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// mediaFamily describes one fal model family reachable through the task
// adaptor. Adding a family is one entry here plus constants; routing is
// derived from the request's media inputs (DC-Media protocol).
type mediaFamily struct {
	Generic string // friendly generic model name, e.g. "h3-max-turbo"

	EndpointT2V string
	EndpointI2V string
	EndpointR2V string // empty when the family has no reference-to-video route

	FirstFrame string // body key for the first frame (i2v)
	LastFrame  string // body key for the last frame (i2v)
	RefImages  string // body key for reference image list (r2v)
	RefAudios  string // body key for reference audio list (r2v)

	AudioT2V string // body key pinning one audio track on t2v/i2v, empty when unsupported
	AudioI2V string // may differ from AudioT2V (turbo: audio_url vs target_audio_url)

	UpperRes bool // resolution enum is "768P" style instead of "768p"

	MinDuration int
	MaxDuration int

	RatioKey  string   // body key for aspect ratio, empty when unsupported
	Passthru  []string // metadata keys copied verbatim into the body
}

var mediaFamilies = []mediaFamily{
	{
		Generic: "h3-max-turbo",
		EndpointT2V: "minimax/h3-max-turbo/text-to-video",
		EndpointI2V: "minimax/h3-max-turbo/image-to-video",
		FirstFrame:  "image_url",
		LastFrame:   "end_image_url",
		AudioT2V:    "target_audio_url",
		AudioI2V:    "target_audio_url",
		UpperRes:    true,
		MinDuration: 2, MaxDuration: 15,
		Passthru: []string{"seed", "enable_safety_checker", "prompt_expansion_mode"},
	},
	{
		Generic: "h3-max",
		EndpointT2V: "minimax/h3-max/text-to-video",
		EndpointI2V: "minimax/h3-max/image-to-video",
		EndpointR2V: "minimax/h3-max/reference-to-video",
		FirstFrame:  "image_url",
		LastFrame:   "end_image_url",
		RefImages:   "image_urls",
		RefAudios:   "audio_urls",
		AudioT2V:    "audio_url",
		AudioI2V:    "target_audio_url",
		UpperRes:    true,
		MinDuration: 2, MaxDuration: 15,
		RatioKey: "aspect_ratio",
		Passthru: []string{"seed", "enable_safety_checker", "prompt_expansion_mode"},
	},
	{
		Generic: "kling-3-pro",
		EndpointT2V: "fal-ai/kling-video/v3/pro/text-to-video",
		EndpointI2V: "fal-ai/kling-video/v3/pro/image-to-video",
		FirstFrame:  "start_image_url",
		LastFrame:   "end_image_url",
		MinDuration: 5, MaxDuration: 10,
		RatioKey: "aspect_ratio",
		Passthru: []string{"seed", "generate_audio", "negative_prompt", "cfg_scale"},
	},
	{
		Generic: "wan-3",
		EndpointT2V: "fal-ai/wan-3/text-to-video",
		EndpointI2V: "fal-ai/wan-3/image-to-video",
		FirstFrame:  "start_image_url",
		LastFrame:   "end_image_url",
		MinDuration: 3, MaxDuration: 10,
		Passthru: []string{"seed", "enable_prompt_expansion", "enable_thinking", "enable_safety_checker", "generate_audio"},
	},
}

// falMediaModels lists friendly names plus raw endpoint ids accepted as models.
func falMediaModels() []string {
	models := []string{
		basefal.ModelH3MaxTurbo, basefal.ModelH3MaxTurboText, basefal.ModelH3MaxTurboImage,
		basefal.ModelH3Max, basefal.ModelH3MaxText, basefal.ModelH3MaxImage, basefal.ModelH3MaxReference,
		basefal.ModelKling3Pro, basefal.ModelKling3ProText, basefal.ModelKling3ProImage,
		basefal.ModelWan3, basefal.ModelWan3Text, basefal.ModelWan3Image,
	}
	for i := range mediaFamilies {
		f := &mediaFamilies[i]
		models = append(models, f.EndpointT2V, f.EndpointI2V)
		if f.EndpointR2V != "" {
			models = append(models, f.EndpointR2V)
		}
	}
	return models
}

func familyForModel(names ...string) *mediaFamily {
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		lower := strings.ToLower(name)
		for i := range mediaFamilies {
			f := &mediaFamilies[i]
			if lower == f.Generic || lower == f.EndpointT2V || lower == f.EndpointI2V ||
				(f.EndpointR2V != "" && lower == f.EndpointR2V) ||
				strings.HasPrefix(lower, f.EndpointT2V+"/") ||
				strings.HasPrefix(lower, f.EndpointI2V+"/") ||
				(f.EndpointR2V != "" && strings.HasPrefix(lower, f.EndpointR2V+"/")) {
				return f
			}
		}
		for i := range mediaFamilies {
			f := &mediaFamilies[i]
			if lower == f.Generic {
				return f
			}
		}
	}
	return nil
}

// variantFamily resolves explicit variant names like "h3-max-text".
func variantFamily(model string) *mediaFamily {
	lower := strings.ToLower(strings.TrimSpace(model))
	for i := range mediaFamilies {
		f := &mediaFamilies[i]
		if lower == f.Generic+"-text" || lower == f.Generic+"/text-to-video" {
			return f
		}
		if lower == f.Generic+"-image" || lower == f.Generic+"/image-to-video" {
			return f
		}
		if f.EndpointR2V != "" && (lower == f.Generic+"-reference" || lower == f.Generic+"/reference-to-video") {
			return f
		}
	}
	return nil
}

func isFalMediaEndpoint(name string) bool {
	if name == "" {
		return false
	}
	lower := strings.ToLower(name)
	for i := range mediaFamilies {
		f := &mediaFamilies[i]
		if lower == f.EndpointT2V || lower == f.EndpointI2V ||
			(f.EndpointR2V != "" && lower == f.EndpointR2V) {
			return true
		}
	}
	return false
}

// mediaRoute is the resolved family route.
type mediaRoute struct {
	Family   *mediaFamily
	Endpoint string
	Action   string // relay action constant name
	UseR2V   bool
	UseI2V   bool
}

// resolveMediaRoute picks the fal endpoint from the request inputs.
// Returns (nil, false) when the model is not one of our families.
func resolveMediaRoute(info *relaycommon.RelayInfo, req relaycommon.TaskSubmitReq) (mediaRoute, bool, error) {
	origin := firstNonEmpty(info.OriginModelName, req.Model)
	upstream := firstNonEmpty(info.UpstreamModelName, origin)

	var family *mediaFamily
	explicitEndpoint := ""
	if isFalMediaEndpoint(upstream) {
		explicitEndpoint = strings.ToLower(upstream)
		family = familyForModel(upstream)
	} else if f := familyForModel(upstream); f != nil {
		family = f
	} else if f := variantFamily(origin); f != nil {
		family = f
	} else if f := familyForModel(origin); f != nil {
		family = f
	} else {
		return mediaRoute{}, false, nil
	}
	if family == nil {
		return mediaRoute{}, false, nil
	}

	images := collectSeedanceImages(req)
	refImages := seedanceReferenceImages(req)
	refVideos := seedanceReferenceVideos(req)
	refAudios := seedanceReferenceAudios(req)
	lastFrame := seedanceLastFrame(req)

	if len(refVideos) > 0 {
		return mediaRoute{}, true, fmt.Errorf(
			"fal %s does not support reference videos; route video conditioning through an editor model",
			family.Generic,
		)
	}

	route := mediaRoute{Family: family}

	// Reference routing: multiple images, or any image+audio mix, or audio-only
	// on a family with a list-audio r2v route, need reference-to-video.
	needR2V := len(refImages) > 1 ||
		(len(refImages) > 0 && len(refAudios) > 0) ||
		(len(refAudios) > 0 && family.EndpointR2V != "" && len(images) == 0 && len(refImages) == 0)

	switch {
	case needR2V:
		if family.EndpointR2V == "" {
			return mediaRoute{}, true, fmt.Errorf(
				"fal %s has no reference-to-video endpoint; use h3-max for mixed references",
				family.Generic,
			)
		}
		route.Endpoint = family.EndpointR2V
		route.Action = constant.TaskActionReferenceGenerate
		route.UseR2V = true
	case len(images) > 0 || lastFrame != "":
		route.Endpoint = family.EndpointI2V
		route.Action = constant.TaskActionFirstTailGenerate
		route.UseI2V = true
	default:
		route.Endpoint = family.EndpointT2V
		route.Action = constant.TaskActionTextGenerate
	}

	if explicitEndpoint != "" {
		// An explicit endpoint id pins the route; validate it belongs to the family.
		belongs := explicitEndpoint == family.EndpointT2V ||
			explicitEndpoint == family.EndpointI2V ||
			explicitEndpoint == family.EndpointR2V
		if !belongs {
			return mediaRoute{}, true, fmt.Errorf(
				"model %q routes to %s but the request inputs fit %s",
				upstream, explicitEndpoint, family.Generic,
			)
		}
		route.Endpoint = explicitEndpoint
		route.UseR2V = explicitEndpoint == family.EndpointR2V
		route.UseI2V = explicitEndpoint == family.EndpointI2V
	}

	// Audio without the r2v route needs a single-track pin field.
	if len(refAudios) > 0 && !route.UseR2V {
		pin := family.AudioT2V
		if route.UseI2V {
			pin = family.AudioI2V
		}
		if pin == "" {
			return mediaRoute{}, true, fmt.Errorf(
				"fal %s does not accept reference audio on this route",
				family.Generic,
			)
		}
	}
	if route.UseR2V && family.RefAudios == "" && len(refAudios) > 0 {
		return mediaRoute{}, true, fmt.Errorf(
			"fal %s does not accept reference audio", family.Generic,
		)
	}
	return route, true, nil
}

// buildMediaRequestBody translates the DC-Media request into the fal payload.
func buildMediaRequestBody(req relaycommon.TaskSubmitReq, route mediaRoute) (map[string]any, error) {
	f := route.Family
	body := map[string]any{"prompt": req.Prompt}

	images := collectSeedanceImages(req)
	refImages := seedanceReferenceImages(req)
	refAudios := seedanceReferenceAudios(req)
	lastFrame := seedanceLastFrame(req)

	if route.UseR2V {
		if len(refImages) > 0 && f.RefImages != "" {
			body[f.RefImages] = refImages
		}
		if len(refAudios) > 0 {
			if f.RefAudios == "" {
				return nil, fmt.Errorf("fal %s does not accept reference audio", f.Generic)
			}
			body[f.RefAudios] = refAudios
		}
		if first := metadataString(req.Metadata, "image_url"); first != "" && len(refImages) == 0 {
			body[f.RefImages] = []string{first}
		}
	} else if route.UseI2V {
		first := metadataString(req.Metadata, "image_url")
		if first == "" && len(images) > 0 {
			first = images[0]
		}
		if first == "" && lastFrame == "" {
			return nil, fmt.Errorf("fal %s image-to-video requires an image", f.Generic)
		}
		if first != "" {
			body[f.FirstFrame] = first
		}
		if lastFrame != "" {
			body[f.LastFrame] = lastFrame
		}
		if len(refAudios) > 0 {
			if f.AudioI2V == "" {
				return nil, fmt.Errorf("fal %s does not accept reference audio on image-to-video", f.Generic)
			}
			body[f.AudioI2V] = refAudios[0]
		}
	} else {
		if len(images) > 0 || lastFrame != "" || len(refImages) > 0 {
			return nil, fmt.Errorf("fal %s text-to-video does not accept media input", f.Generic)
		}
		if len(refAudios) > 0 {
			if f.AudioT2V == "" {
				return nil, fmt.Errorf("fal %s does not accept reference audio", f.Generic)
			}
			body[f.AudioT2V] = refAudios[0]
		}
	}

	if err := applyMediaParams(body, req, route); err != nil {
		return nil, err
	}
	return body, nil
}

func applyMediaParams(body map[string]any, req relaycommon.TaskSubmitReq, route mediaRoute) error {
	f := route.Family
	if duration, ok := resolveSeedanceDuration(req); ok && duration != "auto" {
		seconds, err := strconv.Atoi(duration)
		if err != nil || seconds < f.MinDuration || seconds > f.MaxDuration {
			return fmt.Errorf("fal %s duration %q is not supported; use %d-%d seconds",
				f.Generic, duration, f.MinDuration, f.MaxDuration)
		}
		body["duration"] = seconds
	}
	if resolution, ok := resolveSeedanceResolution(req); ok {
		if f.UpperRes {
			resolution = strings.ToUpper(resolution)
		}
		body["resolution"] = resolution
	}
	if f.RatioKey != "" {
		if ratio, ok := resolveSeedanceAspectRatio(req); ok {
			body[f.RatioKey] = ratio
		}
	}
	for _, key := range f.Passthru {
		if value, ok := req.Metadata[key]; ok {
			if key == "generate_audio" && f.Generic == "wan-3" {
				body["audio"] = value
				continue
			}
			body[key] = value
		}
	}
	return nil
}

// mediaRatios returns billing ratios for family tasks.
func mediaRatios(req relaycommon.TaskSubmitReq, route mediaRoute) map[string]float64 {
	ratios := map[string]float64{}
	if duration, ok := resolveSeedanceDuration(req); ok && duration != "auto" {
		if seconds, err := strconv.Atoi(duration); err == nil && seconds > 0 {
			ratios["seconds"] = float64(seconds)
		}
	}
	switch res := strings.ToLower(mustResolution(req)); res {
	case "480p":
		ratios["resolution"] = 480.0 / 768.0
	case "1080p":
		ratios["resolution"] = 1080.0 / 768.0
	default:
		ratios["resolution"] = 1
	}
	return ratios
}

func mustResolution(req relaycommon.TaskSubmitReq) string {
	resolution, _ := resolveSeedanceResolution(req)
	return resolution
}
