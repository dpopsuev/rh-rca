package rca

import (
	"fmt"
	"log/slog"

	"github.com/dpopsuev/rh-rca/store"
)

// applyStoreEffects updates Store entities based on the completed step's artifact.
func applyStoreEffects(
	st store.Store,
	caseData *store.Case,
	nodeName string,
	artifact any,
) error {
	switch nodeName {
	case "recall":
		return applyRecallEffects(st, caseData, artifact)
	case "triage":
		return applyTriageEffects(st, caseData, artifact)
	case "investigate":
		return applyInvestigateEffects(st, caseData, artifact)
	case "correlate":
		return applyCorrelateEffects(st, caseData, artifact)
	case "review":
		return applyReviewEffects(st, caseData, artifact)
	}
	return nil
}

func applyRecallEffects(st store.Store, caseData *store.Case, artifact any) error {
	m := asMap(artifact)
	if m == nil || !mapBool(m, "match") {
		return nil
	}
	symptomID := mapInt64(m, "symptom_id")
	if symptomID != 0 {
		if err := st.LinkCaseToSymptom(caseData.ID, symptomID); err != nil {
			return fmt.Errorf("link case to symptom: %w", err)
		}
		caseData.SymptomID = symptomID
		_ = st.UpdateSymptomSeen(symptomID)
	}
	priorRCAID := mapInt64(m, "prior_rca_id")
	if priorRCAID != 0 {
		if err := st.LinkCaseToRCA(caseData.ID, priorRCAID); err != nil {
			return fmt.Errorf("link case to rca: %w", err)
		}
		caseData.RCAID = priorRCAID
	}
	return nil
}

func applyTriageEffects(st store.Store, caseData *store.Case, artifact any) error {
	m := asMap(artifact)
	if m == nil {
		return nil
	}
	triage := &store.Triage{
		CaseID:               caseData.ID,
		SymptomCategory:      mapStr(m, "symptom_category"),
		Severity:             mapStr(m, "severity"),
		DefectTypeHypothesis: mapStr(m, "defect_type_hypothesis"),
		SkipInvestigation:    mapBool(m, "skip_investigation"),
		ClockSkewSuspected:   mapBool(m, "clock_skew_suspected"),
		CascadeSuspected:     mapBool(m, "cascade_suspected"),
		DataQualityNotes:     mapStr(m, "data_quality_notes"),
	}
	if _, err := st.CreateTriage(triage); err != nil {
		slog.Warn("create triage failed", "component", "orchestrate", "error", err)
	}

	fingerprint := ComputeFingerprint(caseData.Name, caseData.ErrorMessage, mapStr(m, "symptom_category"))
	sym, err := st.GetSymptomByFingerprint(fingerprint)
	if err != nil {
		slog.Warn("get symptom by fingerprint failed", "component", "orchestrate", "error", err)
	}
	if sym == nil {
		newSym := &store.Symptom{
			Name:            caseData.Name,
			Fingerprint:     fingerprint,
			ErrorPattern:    caseData.ErrorMessage,
			Component:       mapStr(m, "symptom_category"),
			Status:          "active",
			OccurrenceCount: 1,
		}
		symID, err := st.CreateSymptom(newSym)
		if err != nil {
			return fmt.Errorf("create symptom: %w", err)
		}
		caseData.SymptomID = symID
	} else {
		_ = st.UpdateSymptomSeen(sym.ID)
		caseData.SymptomID = sym.ID
	}

	if caseData.SymptomID != 0 {
		if err := st.LinkCaseToSymptom(caseData.ID, caseData.SymptomID); err != nil {
			slog.Warn("link case to symptom failed", "component", "orchestrate", "error", err)
		}
	}
	if err := st.UpdateCaseStatus(caseData.ID, "triaged"); err != nil {
		return fmt.Errorf("update case status after triage: %w", err)
	}
	caseData.Status = "triaged"
	return nil
}

func applyInvestigateEffects(st store.Store, caseData *store.Case, artifact any) error {
	m := asMap(artifact)
	if m == nil {
		return nil
	}
	title := mapStr(m, "rca_message")
	if len(title) > 80 {
		title = title[:80] + "..."
	}
	if title == "" {
		title = "RCA from investigation"
	}
	rca := &store.RCA{
		Title:            title,
		Description:      mapStr(m, "rca_message"),
		DefectType:       mapStr(m, "defect_type"),
		Component:        mapStr(m, "component"),
		ConvergenceScore: mapFloat(m, "convergence_score"),
		Status:           "open",
	}
	rcaID, err := st.SaveRCA(rca)
	if err != nil {
		return fmt.Errorf("save rca: %w", err)
	}

	if err := st.LinkCaseToRCA(caseData.ID, rcaID); err != nil {
		return fmt.Errorf("link case to rca: %w", err)
	}
	if err := st.UpdateCaseStatus(caseData.ID, "investigated"); err != nil {
		return fmt.Errorf("update case status: %w", err)
	}
	caseData.RCAID = rcaID
	caseData.Status = "investigated"

	if caseData.SymptomID != 0 {
		link := &store.SymptomRCA{
			SymptomID:  caseData.SymptomID,
			RCAID:      rcaID,
			Confidence: mapFloat(m, "convergence_score"),
			Notes:      "linked from F3 investigation",
		}
		if _, err := st.LinkSymptomToRCA(link); err != nil {
			slog.Warn("link symptom to RCA failed", "component", "orchestrate", "error", err)
		}
	}
	return nil
}

func applyCorrelateEffects(st store.Store, caseData *store.Case, artifact any) error {
	m := asMap(artifact)
	if m == nil || !mapBool(m, "is_duplicate") {
		return nil
	}
	linkedRCAID := mapInt64(m, "linked_rca_id")
	if linkedRCAID == 0 {
		return nil
	}
	if err := st.LinkCaseToRCA(caseData.ID, linkedRCAID); err != nil {
		return fmt.Errorf("link case to shared rca: %w", err)
	}
	caseData.RCAID = linkedRCAID

	if caseData.SymptomID != 0 {
		link := &store.SymptomRCA{
			SymptomID:  caseData.SymptomID,
			RCAID:      linkedRCAID,
			Confidence: mapFloat(m, "confidence"),
			Notes:      "linked from F4 correlation",
		}
		if _, err := st.LinkSymptomToRCA(link); err != nil {
			slog.Warn("link symptom to RCA failed (correlate)", "component", "orchestrate", "error", err)
		}
	}
	return nil
}

func applyReviewEffects(st store.Store, caseData *store.Case, artifact any) error {
	m := asMap(artifact)
	if m == nil {
		return nil
	}
	decision := mapStr(m, "decision")
	if decision == "approve" {
		if err := st.UpdateCaseStatus(caseData.ID, "reviewed"); err != nil {
			return fmt.Errorf("update case after review: %w", err)
		}
		caseData.Status = "reviewed"
	}
	if decision == "overturn" {
		override := mapMap(m, "human_override")
		if override != nil && caseData.RCAID != 0 {
			rca, err := st.GetRCA(caseData.RCAID)
			if err == nil && rca != nil {
				rca.Description = mapStr(override, "rca_message")
				rca.DefectType = mapStr(override, "defect_type")
				if _, err := st.SaveRCA(rca); err != nil {
					slog.Warn("update RCA after overturn failed", "component", "orchestrate", "error", err)
				}
			}
		}
		if err := st.UpdateCaseStatus(caseData.ID, "reviewed"); err != nil {
			return fmt.Errorf("update case after overturn: %w", err)
		}
		caseData.Status = "reviewed"
	}
	return nil
}

// ComputeFingerprint generates a deterministic fingerprint from failure attributes.
func ComputeFingerprint(testName, errorMessage, component string) string {
	input := testName + "|" + errorMessage + "|" + component
	var h uint64 = 14695981039346656037
	for i := 0; i < len(input); i++ {
		h ^= uint64(input[i])
		h *= 1099511628211
	}
	return fmt.Sprintf("%016x", h)
}
