package data

import (
	"encoding/json"
	"net/http"
	"strings"
)

// HandleListPolicies lists all Row-Level Security policies on a table.
func (controlPlaneHandler *ControlPlaneHandler) HandleListPolicies(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.read") {
		controlPlaneHandler.writeForbidden(responseWriter, request)
		return
	}
	schema, table := controlPlaneHandler.extractSchemaAndTable(request)
	if schema == "" || table == "" {
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, "URL format must be /api/v1/_/data/tables/{schema}/{table}/policies")
		return
	}
	policies, err := controlPlaneHandler.ddlEngine.ListPolicies(request.Context(), schema, table)
	if err != nil {
		controlPlaneHandler.writeError(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, ListPoliciesResponse{
		Policies: policies,
		Count:    len(policies),
	})
}

// HandleCreatePolicy creates a new Row-Level Security policy on a table.
func (controlPlaneHandler *ControlPlaneHandler) HandleCreatePolicy(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.write") {
		controlPlaneHandler.writeForbidden(responseWriter, request)
		return
	}
	schema, table := controlPlaneHandler.extractSchemaAndTable(request)
	if schema == "" || table == "" {
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, "URL format must be /api/v1/_/data/tables/{schema}/{table}/policies")
		return
	}
	var createPolicyRequest CreatePolicyRequest
	if decodeErr := json.NewDecoder(request.Body).Decode(&createPolicyRequest); decodeErr != nil {
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	if createErr := controlPlaneHandler.ddlEngine.CreatePolicy(request.Context(), schema, table, createPolicyRequest); createErr != nil {
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, createErr.Error())
		return
	}

	if controlPlaneHandler.eventBus != nil {
		tableID := schema + "." + table
		controlPlaneHandler.eventBus.Publish(request.Context(), NewTableUpdatedEvent(tableID, TableUpdatedEventData{
			Schema: schema,
			Table:  table,
			Action: "create_policy",
			Detail: createPolicyRequest.Name,
		}))
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusCreated, CreatePolicyResponse{
		Status: "created",
		Policy: createPolicyRequest.Name,
	})
}

// HandleDropPolicy drops a Row-Level Security policy from a table.
func (controlPlaneHandler *ControlPlaneHandler) HandleDropPolicy(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.write") {
		controlPlaneHandler.writeForbidden(responseWriter, request)
		return
	}
	schema, table := controlPlaneHandler.extractSchemaAndTable(request)
	policyName := controlPlaneHandler.extractPolicyName(request)
	if schema == "" || table == "" || policyName == "" {
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, "URL format must be /api/v1/_/data/tables/{schema}/{table}/policies/{policy}")
		return
	}
	if dropErr := controlPlaneHandler.ddlEngine.DropPolicy(request.Context(), schema, table, policyName); dropErr != nil {
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, dropErr.Error())
		return
	}

	if controlPlaneHandler.eventBus != nil {
		tableID := schema + "." + table
		controlPlaneHandler.eventBus.Publish(request.Context(), NewTableUpdatedEvent(tableID, TableUpdatedEventData{
			Schema: schema,
			Table:  table,
			Action: "drop_policy",
			Detail: policyName,
		}))
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, DropPolicyResponse{
		Status: "deleted",
		Policy: policyName,
	})
}

// HandleToggleRLS toggles Row-Level Security mode (ENABLE / DISABLE / FORCE / UNFORCE).
func (controlPlaneHandler *ControlPlaneHandler) HandleToggleRLS(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.write") {
		controlPlaneHandler.writeForbidden(responseWriter, request)
		return
	}
	schema, table := controlPlaneHandler.extractSchemaAndTable(request)
	if schema == "" || table == "" {
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, "URL format must be /api/v1/_/data/tables/{schema}/{table}/rls")
		return
	}

	action := strings.ToUpper(strings.TrimSpace(request.URL.Query().Get("action")))
	if action == "" {
		path := strings.TrimSuffix(request.URL.Path, "/")
		if strings.HasSuffix(path, "/enable") {
			action = "ENABLE"
		} else if strings.HasSuffix(path, "/disable") {
			action = "DISABLE"
		} else if strings.HasSuffix(path, "/force") {
			action = "FORCE"
		}
	}
	if action == "" && request.Body != nil {
		var toggleTableRLSRequest ToggleTableRLSRequest
		if decodeErr := json.NewDecoder(request.Body).Decode(&toggleTableRLSRequest); decodeErr == nil {
			if toggleTableRLSRequest.Action != "" {
				action = strings.ToUpper(strings.TrimSpace(toggleTableRLSRequest.Action))
			} else if toggleTableRLSRequest.Mode != "" {
				action = strings.ToUpper(strings.TrimSpace(toggleTableRLSRequest.Mode))
			}
		}
	}

	switch action {
	case "ENABLE":
		if enableErr := controlPlaneHandler.ddlEngine.EnableRLS(request.Context(), schema, table); enableErr != nil {
			controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, enableErr.Error())
			return
		}
		if controlPlaneHandler.eventBus != nil {
			controlPlaneHandler.eventBus.Publish(request.Context(), NewTableUpdatedEvent(schema+"."+table, TableUpdatedEventData{
				Schema: schema,
				Table:  table,
				Action: "toggle_rls",
				Detail: "enabled",
			}))
		}
		controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, ToggleTableRLSResponse{
			Status: "enabled",
			Mode:   "ENABLE",
		})
	case "DISABLE":
		if disableErr := controlPlaneHandler.ddlEngine.DisableRLS(request.Context(), schema, table); disableErr != nil {
			controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, disableErr.Error())
			return
		}
		if controlPlaneHandler.eventBus != nil {
			controlPlaneHandler.eventBus.Publish(request.Context(), NewTableUpdatedEvent(schema+"."+table, TableUpdatedEventData{
				Schema: schema,
				Table:  table,
				Action: "toggle_rls",
				Detail: "disabled",
			}))
		}
		controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, ToggleTableRLSResponse{
			Status: "disabled",
			Mode:   "DISABLE",
		})
	case "FORCE":
		force := request.URL.Query().Get("force") != "false"
		if forceErr := controlPlaneHandler.ddlEngine.ForceRLS(request.Context(), schema, table, force); forceErr != nil {
			controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, forceErr.Error())
			return
		}
		status := "forced"
		if !force {
			status = "unforced"
		}
		if controlPlaneHandler.eventBus != nil {
			controlPlaneHandler.eventBus.Publish(request.Context(), NewTableUpdatedEvent(schema+"."+table, TableUpdatedEventData{
				Schema: schema,
				Table:  table,
				Action: "toggle_rls",
				Detail: status,
			}))
		}
		controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, ToggleTableRLSResponse{
			Status: status,
			Mode:   "FORCE",
		})
	case "UNFORCE":
		if unforceErr := controlPlaneHandler.ddlEngine.ForceRLS(request.Context(), schema, table, false); unforceErr != nil {
			controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, unforceErr.Error())
			return
		}
		if controlPlaneHandler.eventBus != nil {
			controlPlaneHandler.eventBus.Publish(request.Context(), NewTableUpdatedEvent(schema+"."+table, TableUpdatedEventData{
				Schema: schema,
				Table:  table,
				Action: "toggle_rls",
				Detail: "unforced",
			}))
		}
		controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, ToggleTableRLSResponse{
			Status: "unforced",
			Mode:   "UNFORCE",
		})
	default:
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, "Unknown RLS action")
	}
}
