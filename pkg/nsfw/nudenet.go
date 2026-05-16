// Package nsfw provides a client for an external NSFW image classifier.
//
// We use the open-source NudeNet model served as an HTTP sidecar. The
// upstream Docker image (`ghcr.io/notai-tech/nudenet:latest`) exposes a
// `POST /infer` endpoint that accepts multipart-form file uploads and
// returns a list of detected regions with class labels + confidence.
//
// Detection model (NudeNet v3) classes considered explicit:
//
//	FEMALE_GENITALIA_EXPOSED
//	MALE_GENITALIA_EXPOSED
//	FEMALE_BREAST_EXPOSED
//	BUTTOCKS_EXPOSED
//	ANUS_EXPOSED
//
// Suggestive but not auto-blocked (returned in result so callers can
// decide policy):
//
//	FEMALE_BREAST_COVERED
//	BUTTOCKS_COVERED
//	FEMALE_GENITALIA_COVERED
//
// Network errors and decoding errors are treated as "scan failed, not
// flagged" — uploads are not held hostage to a flaky sidecar. Callers
// can inspect Result.ScannerError if they want stricter behavior.
//
// Deployment (free, self-hosted):
//
//	docker run -d --restart=always --name nudenet \
//	  -p 127.0.0.1:8001:8000 \
//	  ghcr.io/notai-tech/nudenet:latest
//
// Set NSFW_SCANNER_URL=http://127.0.0.1:8001 in the API env and pass
// the resulting client into StorageService.
package nsfw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// explicitClasses are the labels that result in IsExplicit=true when
// any detection's score crosses BlockThreshold. The list is the most
// conservative subset of NudeNet's vocabulary: anything genital,
// breast-exposed, buttocks-exposed, or anus-exposed. Adjust if your
// policy is stricter (e.g. block suggestive-covered too).
var explicitClasses = map[string]struct{}{
	"FEMALE_GENITALIA_EXPOSED": {},
	"MALE_GENITALIA_EXPOSED":   {},
	"FEMALE_BREAST_EXPOSED":    {},
	"BUTTOCKS_EXPOSED":         {},
	"ANUS_EXPOSED":             {},
}

// Client wraps the HTTP API of a NudeNet sidecar.
type Client struct {
	baseURL        string
	httpClient     *http.Client
	BlockThreshold float64
}

// New returns a configured client. baseURL is the sidecar root (e.g.
// http://127.0.0.1:8001). blockThreshold is the per-detection score
// above which a flagged class triggers IsExplicit=true. 0.6 is a
// reasonable default — high enough to dodge most false positives on
// medical / artistic content, low enough to catch obvious cases.
func New(baseURL string, blockThreshold float64) *Client {
	if blockThreshold <= 0 || blockThreshold > 1 {
		blockThreshold = 0.6
	}
	return &Client{
		baseURL:        strings.TrimRight(baseURL, "/"),
		BlockThreshold: blockThreshold,
		httpClient: &http.Client{
			// Scanner is a hot-path dependency — fail fast so a stuck
			// sidecar can't pile up open uploads at the app server.
			Timeout: 5 * time.Second,
		},
	}
}

// Detection is one labeled region from NudeNet.
type Detection struct {
	Class string  `json:"class"`
	Score float64 `json:"score"`
}

// Result is the scan outcome. IsExplicit is true when any Detection
// in an explicit class crosses BlockThreshold. ScannerError is set
// when the HTTP call or response parse failed — callers should treat
// scan-failed as "not flagged" but may log/alert on it.
type Result struct {
	IsExplicit   bool
	TopClass     string
	TopScore     float64
	Detections   []Detection
	ScannerError error
}

// nudeNetResponse mirrors the upstream JSON shape:
//
//	{"prediction": [{"class": "FACE_FEMALE", "score": 0.93}, ...]}
//
// Some forks return `detections` instead of `prediction` — we accept both.
type nudeNetResponse struct {
	Prediction []Detection `json:"prediction"`
	Detections []Detection `json:"detections"`
}

// Scan classifies bytes against the NudeNet sidecar. Pass the
// already-decoded image bytes (after format validation) so the scanner
// works on the same payload that will be uploaded to MinIO.
//
// filename is used only as a multipart-form hint; pass the user-supplied
// filename so logs make sense. contentType should be the canonical MIME
// (e.g. "image/webp"); empty is fine.
func (c *Client) Scan(ctx context.Context, data []byte, filename, contentType string) Result {
	if c == nil || c.baseURL == "" {
		return Result{} // scanner disabled — fail open
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	header := make(map[string][]string)
	header["Content-Disposition"] = []string{
		fmt.Sprintf(`form-data; name="file"; filename=%q`, filenameOrFallback(filename)),
	}
	if contentType != "" {
		header["Content-Type"] = []string{contentType}
	}
	part, err := writer.CreatePart(header)
	if err != nil {
		return Result{ScannerError: fmt.Errorf("nsfw: build multipart: %w", err)}
	}
	if _, err := part.Write(data); err != nil {
		return Result{ScannerError: fmt.Errorf("nsfw: write payload: %w", err)}
	}
	if err := writer.Close(); err != nil {
		return Result{ScannerError: fmt.Errorf("nsfw: close multipart: %w", err)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/infer", body)
	if err != nil {
		return Result{ScannerError: fmt.Errorf("nsfw: build request: %w", err)}
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Result{ScannerError: fmt.Errorf("nsfw: request: %w", err)}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		// Read up to 512 bytes so we don't drag huge HTML error pages.
		preview, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return Result{ScannerError: fmt.Errorf("nsfw: status %d: %s", resp.StatusCode, preview)}
	}

	var parsed nudeNetResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return Result{ScannerError: fmt.Errorf("nsfw: decode: %w", err)}
	}
	dets := parsed.Prediction
	if len(dets) == 0 {
		dets = parsed.Detections
	}

	out := Result{Detections: dets}
	for _, d := range dets {
		if d.Score > out.TopScore {
			out.TopClass = d.Class
			out.TopScore = d.Score
		}
		if _, isExplicit := explicitClasses[d.Class]; isExplicit && d.Score >= c.BlockThreshold {
			out.IsExplicit = true
		}
	}
	return out
}

func filenameOrFallback(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "upload.bin"
	}
	return name
}
