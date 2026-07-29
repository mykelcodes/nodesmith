package services

import (
	"fmt"

	"nodesmith/internal/recipe"
)

// cloneMinutes copies an optional cooldown so callers cannot mutate stored
// state through the returned pointer.
func cloneMinutes(minutes *int) *int {
	if minutes == nil {
		return nil
	}
	value := *minutes
	return &value
}

// resolveMinimumReleaseAge picks the cooldown that applies to one scaffold
// request. A value selected during catalogue configuration beats the recipe
// manifest, which in turn beats the global preference. An explicit zero counts
// as a value and disables the cooldown for that layer and below.
func resolveMinimumReleaseAge(
	requestMinutes *int,
	settings Settings,
	manifest recipe.Manifest,
) MinimumReleaseAgeResolution {
	if requestMinutes != nil {
		return MinimumReleaseAgeResolution{
			Minutes: cloneMinutes(requestMinutes),
			Source:  ReleaseAgeSourceRequest,
		}
	}
	if manifest.MinimumReleaseAge != nil {
		return MinimumReleaseAgeResolution{
			Minutes: cloneMinutes(manifest.MinimumReleaseAge),
			Source:  ReleaseAgeSourceRecipe,
		}
	}
	if settings.MinimumReleaseAge != nil {
		return MinimumReleaseAgeResolution{
			Minutes: cloneMinutes(settings.MinimumReleaseAge),
			Source:  ReleaseAgeSourceGlobal,
		}
	}
	return MinimumReleaseAgeResolution{Source: ReleaseAgeSourceUnset}
}

// validateMinimumReleaseAgeSettings checks the global value against the range
// recipe manifests and catalogue configuration must also satisfy.
func validateMinimumReleaseAgeSettings(settings Settings) error {
	if settings.MinimumReleaseAge != nil {
		if err := recipe.ValidateMinimumReleaseAge(*settings.MinimumReleaseAge); err != nil {
			return fmt.Errorf("minimum release age: %w", err)
		}
	}
	return nil
}

func validateMinimumReleaseAgeRequest(request ScaffoldRequest) error {
	if request.MinimumReleaseAge == nil {
		return nil
	}
	if err := recipe.ValidateMinimumReleaseAge(*request.MinimumReleaseAge); err != nil {
		return fmt.Errorf("minimum release age: %w", err)
	}
	return nil
}
