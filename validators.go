// Copyright (c) 2026 Arsenii Kvachan
// SPDX-License-Identifier: Unlicense

package hirevec

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	RegexHTMLTag   = regexp.MustCompile(`(?s)<[^>]*>`)
	RegexJSTag     = regexp.MustCompile(`(?is)<script.*?>.*?</script>`)
	RegexSQL       = regexp.MustCompile(`(?i)\b(select|insert|update|delete|drop|truncate|alter)\b`)
	RegexURL       = regexp.MustCompile(`https?://|www\.`)
	RegexWebScheme = regexp.MustCompile(`(?i)^(https?|ftp|ftps|ws|wss|file|data|javascript|vbscript):`)
)

func NormalizeAndValidateCandidateExperiences(experiences []RequestBodyCandidateExperience) ([]CandidateExperience, error) {
	normalized := make([]CandidateExperience, 0, len(experiences))

	var validationErrors []error
	for i, experience := range experiences {
		title, err := NormalizeAndValidateText(experience.Title, "title", DefaultExperienceTitleMinLength, DefaultExperienceTitleMaxLength, true)
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("%w: experiences/%v/title", err, i))
		}

		experienceType := ExperienceType(experience.ExperienceType)
		if !experienceType.IsValid() {
			validationErrors = append(validationErrors, fmt.Errorf("%w: experiences/%v/experience_type", ErrInvalidExperienceType, i))
		}

		if experience.StartedAt.After(experience.EndedAt) {
			validationErrors = append(validationErrors, fmt.Errorf("%w: experiences/%v/started_at", ErrInvalidExperienceDates, i))
		}

		description, err := NormalizeAndValidateText(experience.Description, "description", DefaultExperienceDescriptionMinLength, DefaultExperienceDescriptionMaxLength, false)
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("%w: experiences/%v/description", err, i))
		}

		company, err := NormalizeAndValidateText(experience.Company, "company", DefaultPositionCompanyMinLength, DefaultPositionCompanyMaxLength, true)
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("%w: experiences/%v/company", err, i))
		}

		skills := []string{experience.Skill1, experience.Skill2, experience.Skill3, experience.Skill4, experience.Skill5}
		for j, skill := range skills {
			if skill, err = NormalizeAndValidateText(skill, "skill", DefaultSkillMinLength, DefaultSkillMaxLength, false); err != nil {
				validationErrors = append(validationErrors, fmt.Errorf("%w: experiences/%v/skill_%v", err, i, j+1))
			}
			skills[j] = skill
		}

		normalized = append(normalized, CandidateExperience{
			Title:          title,
			StartedAt:      experience.StartedAt,
			EndedAt:        experience.EndedAt,
			Description:    description,
			Company:        company,
			ExperienceType: experienceType,
			Skill1:         skills[0],
			Skill2:         skills[1],
			Skill3:         skills[2],
			Skill4:         skills[3],
			Skill5:         skills[4],
		})
	}

	return normalized, errors.Join(validationErrors...)
}

func NormalizeAndValidateText(text string, textType string, minLength int, maxLength int, removeSpaces bool) (string, error) {
	text = strings.TrimSpace(text)
	if removeSpaces {
		text = strings.Join(strings.Fields(text), " ")
	}

	var validationErrors []error
	if len(text) < minLength || len(text) > maxLength {
		validationErrors = append(validationErrors, fmt.Errorf("%s must be between %d and %d characters", textType, minLength, maxLength))
	}
	if RegexWebScheme.MatchString(text) || RegexURL.MatchString(text) {
		validationErrors = append(validationErrors, fmt.Errorf("%s must not contain URLs", textType))
	}

	return text, errors.Join(validationErrors...)
}
