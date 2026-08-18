package admin

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/enterpilot/gomodel/internal/core"
	"github.com/enterpilot/gomodel/internal/thinkingoverrides"
)

type upsertModelThinkingOverrideRequest struct {
	Selector string                   `json:"selector"`
	Mode     thinkingoverrides.Mode   `json:"mode"`
	Thinking *bool                    `json:"thinking"`
	Effort   string                   `json:"effort"`
}

type deleteModelThinkingOverrideRequest struct {
	Selector string `json:"selector"`
}

// ListModelThinkingOverrides handles GET /admin/model-thinking-overrides.
//
// @Summary      List model thinking overrides
// @Description  Lists per-model thinking overrides (native/supplement/override, thinking on/off, effort level).
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   thinkingoverrides.Override
// @Failure      401  {object}  core.GatewayError
// @Failure      503  {object}  core.GatewayError
// @Router       /admin/model-thinking-overrides [get]
func (h *Handler) ListModelThinkingOverrides(c *echo.Context) error {
	if h.thinkingOverrides == nil {
		return handleError(c, featureUnavailableError("model thinking overrides feature is unavailable"))
	}
	overrides := h.thinkingOverrides.List()
	if overrides == nil {
		overrides = []thinkingoverrides.Override{}
	}
	return c.JSON(http.StatusOK, overrides)
}

// UpsertModelThinkingOverride handles PUT /admin/model-thinking-overrides.
//
// @Summary      Create or update one model thinking override
// @Description  Stores thinking override settings for one model selector. Mode "native" leaves requests untouched; "supplement" fills unset thinking parameters; "override" forces them.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        override  body      upsertModelThinkingOverrideRequest  true  "Thinking selector and override"
// @Success      200       {object}  thinkingoverrides.Override
// @Failure      400       {object}  core.GatewayError
// @Failure      401       {object}  core.GatewayError
// @Failure      500       {object}  core.GatewayError
// @Failure      502       {object}  core.GatewayError
// @Failure      503       {object}  core.GatewayError
// @Router       /admin/model-thinking-overrides [put]
func (h *Handler) UpsertModelThinkingOverride(c *echo.Context) error {
	if h.thinkingOverrides == nil {
		return handleError(c, featureUnavailableError("model thinking overrides feature is unavailable"))
	}

	var req upsertModelThinkingOverrideRequest
	if err := c.Bind(&req); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body: "+err.Error(), err))
	}
	selector, err := normalizeModelPricingOverrideSelector(req.Selector)
	if err != nil {
		return handleError(c, err)
	}
	thinking := true
	if req.Thinking != nil {
		thinking = *req.Thinking
	}

	if err := h.thinkingOverrides.Upsert(c.Request().Context(), thinkingoverrides.Override{
		Selector: selector,
		Mode:     req.Mode,
		Thinking: thinking,
		Effort:   req.Effort,
	}); err != nil {
		return handleError(c, thinkingOverrideWriteError(err))
	}

	override, ok := h.thinkingOverrides.Get(selector)
	if !ok {
		slog.Error("model thinking override service returned no override after upsert", "selector", selector)
		return handleError(c, core.NewProviderError("model_thinking_overrides", http.StatusInternalServerError, "model thinking override update failed unexpectedly", nil))
	}
	return c.JSON(http.StatusOK, override)
}

// DeleteModelThinkingOverride handles DELETE /admin/model-thinking-overrides.
//
// @Summary      Delete one model thinking override
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  deleteModelThinkingOverrideRequest  true  "Thinking selector to remove"
// @Success      204       "No Content"
// @Failure      400       {object}  core.GatewayError
// @Failure      401       {object}  core.GatewayError
// @Failure      404       {object}  core.GatewayError
// @Failure      502       {object}  core.GatewayError
// @Failure      503       {object}  core.GatewayError
// @Router       /admin/model-thinking-overrides [delete]
func (h *Handler) DeleteModelThinkingOverride(c *echo.Context) error {
	if h.thinkingOverrides == nil {
		return handleError(c, featureUnavailableError("model thinking overrides feature is unavailable"))
	}

	var req deleteModelThinkingOverrideRequest
	if err := c.Bind(&req); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body: "+err.Error(), err))
	}
	selector, err := normalizeModelPricingOverrideSelector(req.Selector)
	if err != nil {
		return handleError(c, err)
	}

	if err := h.thinkingOverrides.Delete(c.Request().Context(), selector); err != nil {
		if errors.Is(err, thinkingoverrides.ErrNotFound) {
			return handleError(c, core.NewNotFoundError("model thinking override not found: "+selector))
		}
		return handleError(c, thinkingOverrideWriteError(err))
	}
	return c.NoContent(http.StatusNoContent)
}
