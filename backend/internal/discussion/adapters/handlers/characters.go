package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/an4eetos/serendipity/backend/internal/discussion/adapters/converters"
	"github.com/an4eetos/serendipity/backend/internal/discussion/usecase"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/httpapi"
)

var _ httpapi.Routes = (*Characters)(nil)

// Characters serves readers' management of their own characters. Only the
// owner gets these responses: they carry the prompt.
type Characters struct {
	list      *usecase.ListCharacters
	create    *usecase.CreateCharacter
	update    *usecase.UpdateCharacter
	archive   *usecase.ArchiveCharacter
	unarchive *usecase.UnarchiveCharacter
}

// NewCharacters builds the handler of readers' own characters.
func NewCharacters(list *usecase.ListCharacters, create *usecase.CreateCharacter, update *usecase.UpdateCharacter,
	archive *usecase.ArchiveCharacter, unarchive *usecase.UnarchiveCharacter) *Characters {
	return &Characters{list: list, create: create, update: update, archive: archive, unarchive: unarchive}
}

// RegisterRoutes mounts the character routes, all on the private group.
func (h *Characters) RegisterRoutes(_, private chi.Router) {
	private.Get("/characters", h.listCharacters)
	private.Post("/characters", h.createCharacter)
	private.Patch("/characters/{id}", h.updateCharacter)
	private.Post("/characters/{id}/archive", h.archiveCharacter)
	private.Post("/characters/{id}/unarchive", h.unarchiveCharacter)
}

func (h *Characters) listCharacters(w http.ResponseWriter, r *http.Request) {
	l, err := h.list.Execute(r.Context(), httpapi.UserID(r.Context()))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, converters.ToCharacterListResponse(l))
}

func (h *Characters) createCharacter(w http.ResponseWriter, r *http.Request) {
	var req converters.CharacterRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	p, err := h.create.Execute(r.Context(), httpapi.UserID(r.Context()), req.ToDraft())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, converters.ToCharacterResponse(p))
}

func (h *Characters) updateCharacter(w http.ResponseWriter, r *http.Request) {
	var req converters.CharacterPatchRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	p, err := h.update.Execute(r.Context(), httpapi.UserID(r.Context()), chi.URLParam(r, "id"), req.ToPatch())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, converters.ToCharacterResponse(p))
}

func (h *Characters) archiveCharacter(w http.ResponseWriter, r *http.Request) {
	p, err := h.archive.Execute(r.Context(), httpapi.UserID(r.Context()), chi.URLParam(r, "id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, converters.ToCharacterResponse(p))
}

func (h *Characters) unarchiveCharacter(w http.ResponseWriter, r *http.Request) {
	p, err := h.unarchive.Execute(r.Context(), httpapi.UserID(r.Context()), chi.URLParam(r, "id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, converters.ToCharacterResponse(p))
}
