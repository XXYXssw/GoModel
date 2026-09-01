package server

import (
	"bytes"
	"context"
	"io"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/enterpilot/gomodel/internal/auditlog"
	"github.com/enterpilot/gomodel/internal/core"
	"github.com/enterpilot/gomodel/internal/session"
)

const interactionParentHeader = "X-GoModel-Interaction-Parent"

// sessionBodyCaptureLimit bounds how much of a request body is read for
// session detection. Agent clients (DeepSeek Harness, Claude Code, Codex)
// routinely send 200KB–2MB chat bodies; the previous cap (auditlog.MaxBodyCapture,
// 256KB) made every large conversation lose its auto-detected session, so the
// dashboard grouped those requests as one-off conversations. Session detection
// only needs the conversation prefix (model + system + opening messages), which
// sits at the start of the body, so 2MB covers real-world agent payloads.
// Bodies beyond auditlog.MaxBodyCapture are still dropped from the snapshot
// after detection, keeping the audit "body too big" behavior and storage cap
// exactly as before.
const sessionBodyCaptureLimit = 8 * auditlog.MaxBodyCapture // 2MB

type interactionParentLookup interface {
	GetInteractionParent(ctx context.Context, id string) (*auditlog.InteractionParent, error)
}

// sessionCapture resolves session identity after authentication and before
// routing. Dashboard continuations inherit their persisted parent's resolved
// session; ordinary requests use the configured detector.
func sessionCapture(detector *session.Detector, parentLookup interactionParentLookup, authMiddlewareAbsent bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if detector == nil {
				return next(c)
			}
			snapshot := core.GetRequestSnapshot(c.Request().Context())
			if snapshot == nil || !core.IsModelInteractionPath(snapshot.Path) {
				return next(c)
			}
			stamp := func(id string) bool {
				id = strings.TrimSpace(id)
				if id == "" {
					return false
				}
				req := c.Request()
				c.SetRequest(req.WithContext(core.WithSessionID(req.Context(), id)))
				auditlog.EnrichEntryWithSessionID(c, id)
				return true
			}
			detectAndStamp := func(snapshot *core.RequestSnapshot) bool {
				return stamp(detector.Detect(snapshot, core.UserPathFromContext(c.Request().Context())))
			}
			if stamp(interactionParentSession(c, parentLookup, authMiddlewareAbsent)) {
				return next(c)
			}

			// A captured body lets the detector resolve every rule in one pass.
			if snapshot.CapturedBodyView() != nil {
				detectAndStamp(snapshot)
				return next(c)
			}

			// Header rules do not need the body. Resolve them first so an
			// explicit session header keeps large/chunked requests on the
			// zero-copy path.
			if detectAndStamp(snapshot) {
				return next(c)
			}

			var err error
			var oversized bool
			snapshot, oversized, err = sessionDetectionSnapshot(c, snapshot)
			if err != nil {
				return handleError(c, core.NewInvalidRequestError("failed to read request body", err))
			}
			detectAndStamp(snapshot)
			// Session detection used the captured body; for bodies beyond the
			// audit capture cap, drop the body afterwards so audit logging keeps
			// its historical "too big to handle" behavior. The stamped session
			// id lives in the request context, not in the captured body.
			if oversized {
				snapshot = markSessionBodyNotCaptured(c, snapshot)
			}
			return next(c)
		}
	}
}

func interactionParentSession(c *echo.Context, lookup interactionParentLookup, allowWithoutAuth bool) string {
	if c == nil || lookup == nil ||
		(!allowWithoutAuth && !interactionContinuationAllowed(c.Request().Context())) {
		return ""
	}
	parentID := strings.TrimSpace(c.Request().Header.Get(interactionParentHeader))
	if parentID == "" || len(parentID) > 200 || strings.ContainsAny(parentID, ",\x00") {
		return ""
	}
	parent, err := lookup.GetInteractionParent(c.Request().Context(), parentID)
	if err != nil || parent == nil {
		return ""
	}
	parentPath := strings.TrimSpace(parent.UserPath)
	if parentPath == "" {
		parentPath = "/"
	}
	requestPath := strings.TrimSpace(core.UserPathFromContext(c.Request().Context()))
	if requestPath == "" {
		requestPath = "/"
	}
	if parentPath != requestPath {
		return ""
	}
	return strings.TrimSpace(parent.SessionID)
}

// sessionDetectionSnapshot returns a snapshot whose complete body is available
// for session detection, up to sessionBodyCaptureLimit. It never reads a
// known-oversized body and peeks only limit+1 bytes from an unknown-length
// body. Oversized bodies are replayed intact for the handler and fall back to
// header signals. The returned oversized flag reports that the body exceeded
// auditlog.MaxBodyCapture, so the caller can drop it from the snapshot after
// detection and keep audit logging's storage behavior unchanged.
func sessionDetectionSnapshot(c *echo.Context, snapshot *core.RequestSnapshot) (*core.RequestSnapshot, bool, error) {
	switch core.DescribeEndpoint(snapshot.Method, snapshot.Path).BodyMode {
	case core.BodyModeJSON, core.BodyModeOpaque:
	default:
		return snapshot, false, nil
	}
	req := c.Request()
	if req.Body == nil {
		return snapshot, false, nil
	}
	if req.ContentLength > sessionBodyCaptureLimit {
		return markSessionBodyNotCaptured(c, snapshot), true, nil
	}

	originalBody := req.Body
	body, err := io.ReadAll(io.LimitReader(originalBody, sessionBodyCaptureLimit+1))
	if err != nil {
		// Preserve the bytes already consumed even though this request will be
		// rejected, keeping the helper's ownership contract explicit.
		req.Body = &combinedReadCloser{
			Reader: io.MultiReader(bytes.NewReader(body), originalBody),
			rc:     originalBody,
		}
		return snapshot, false, err
	}
	if int64(len(body)) > sessionBodyCaptureLimit {
		req.Body = &combinedReadCloser{
			Reader: io.MultiReader(bytes.NewReader(body), originalBody),
			rc:     originalBody,
		}
		return markSessionBodyNotCaptured(c, snapshot), true, nil
	}

	// The full body fit. Cache it on the shared snapshot and replay the same
	// bytes to downstream code without another read or allocation.
	req.Body = &combinedReadCloser{Reader: bytes.NewReader(body), rc: originalBody}
	storeRequestBodySnapshot(c, body)
	if refreshed := core.GetRequestSnapshot(c.Request().Context()); refreshed != nil {
		snapshot = refreshed
	}
	// Bodies beyond the audit capture cap were captured only for session
	// detection; the caller drops them from the snapshot after stamping so
	// audit logging keeps its historical "too big to handle" behavior instead
	// of storing multi-MB payloads.
	return snapshot, int64(len(body)) > auditlog.MaxBodyCapture, nil
}

func markSessionBodyNotCaptured(c *echo.Context, snapshot *core.RequestSnapshot) *core.RequestSnapshot {
	updated := snapshot.WithOwnedCapturedBody(nil, true)
	req := c.Request()
	c.SetRequest(req.WithContext(core.WithRequestSnapshot(req.Context(), updated)))
	return updated
}
