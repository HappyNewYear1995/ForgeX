package service

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"jenkinsAgent/internal/model"
	"jenkinsAgent/internal/store"
)

// PackageService handles aggregation of component artifacts into final packages
type PackageService struct {
	artifactStore *store.ArtifactStore
	buildStore    *store.BuildStore
}

func NewPackageService() *PackageService {
	return &PackageService{
		artifactStore: store.NewArtifactStore(),
		buildStore:    store.NewBuildStore(),
	}
}

// PackageRelease aggregates all successful build artifacts for a release+buildType
// into a structured zip file: svr/, web/, manifest.json
// Naming: acis_{code}_{version}_{buildEnv}.zip
func (s *PackageService) PackageRelease(release *model.Release, buildEnv string, productCode string, compTypes map[string]model.ComponentType) (string, error) {
	// Use app dir (exe location) for package storage
	appDir := getAppDir()
	packageDir := filepath.Join(appDir, "data", "packages")
	_ = os.MkdirAll(packageDir, 0755)
	zipName := fmt.Sprintf("acis_%s_%s_%s.zip", productCode, release.Version, buildEnv)
	zipPath := filepath.Join(packageDir, zipName)
	if _, err := os.Stat(zipPath); err == nil {
		return zipPath, nil // already packaged
	}

	// Find all successful builds for this release with matching buildEnv
	builds, err := s.buildStore.ListByProductID(release.ProductID)
	if err != nil {
		return "", fmt.Errorf("list builds error: %w", err)
	}

	var matchingBuilds []model.Build
	for _, b := range builds {
		if b.ReleaseID == release.ID && b.BuildEnv == buildEnv && b.Status == model.BuildStatusSuccess {
			matchingBuilds = append(matchingBuilds, b)
		}
	}

	if len(matchingBuilds) == 0 {
		return "", fmt.Errorf("没有成功的 %s 构建", buildEnv)
	}

	// Collect all artifacts from matching builds
	var allArtifacts []model.Artifact
	for _, b := range matchingBuilds {
		artifacts, err := s.artifactStore.ListByBuildID(b.ID)
		if err != nil {
			log.Printf("[package] error listing artifacts for build#%d: %v", b.ID, err)
			continue
		}
		allArtifacts = append(allArtifacts, artifacts...)
	}

	if len(allArtifacts) == 0 {
		return "", fmt.Errorf("没有找到制品文件")
	}

	// Create zip file with structured layout
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("create zip error: %w", err)
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)
	defer zw.Close()

	// Create directory entries
	if _, err := zw.Create("svr/"); err != nil {
		log.Printf("[package] create svr/ dir error: %v", err)
	}
	if _, err := zw.Create("web/"); err != nil {
		log.Printf("[package] create web/ dir error: %v", err)
	}
	if _, err := zw.Create("other/"); err != nil {
		log.Printf("[package] create other/ dir error: %v", err)
	}

	addedFiles := make(map[string]bool)
	for _, artifact := range allArtifacts {
		// Resolve artifact path relative to appDir if not absolute
		artPath := artifact.FilePath
		if !filepath.IsAbs(artPath) {
			artPath = filepath.Join(appDir, artPath)
		}
		if _, err := os.Stat(artPath); os.IsNotExist(err) {
			log.Printf("[package] artifact file not found: %s", artPath)
			continue
		}

		// Determine target directory based on component type
		targetDir := "svr" // default to backend
		if ct, ok := compTypes[artifact.ComponentName]; ok {
			switch ct {
			case model.ComponentTypeFrontend:
				targetDir = "web"
			case model.ComponentTypeOther:
				targetDir = "other"
			}
		}

		// Try to extract zip contents into the target directory
		extracted, err := s.extractArtifactToZip(zw, artPath, artifact.FileName, targetDir, addedFiles)
		if err != nil {
			log.Printf("[package] extract artifact %s error: %v (copying as-is)", artifact.FileName, err)
		}
		if !extracted {
			// Not a zip or extraction failed - copy as-is to target dir
			entryName := fmt.Sprintf("%s/%s", targetDir, artifact.FileName)
			if addedFiles[entryName] {
				entryName = fmt.Sprintf("%s/build_%d_%s", targetDir, artifact.BuildID, artifact.FileName)
			}
			addedFiles[entryName] = true

			fw, err := zw.Create(entryName)
			if err != nil {
				log.Printf("[package] zip create entry error: %v", err)
				continue
			}
			src, err := os.Open(artPath)
			if err != nil {
				log.Printf("[package] open artifact error: %v", err)
				continue
			}
			if _, err := io.Copy(fw, src); err != nil {
				log.Printf("[package] copy artifact error: %v", err)
			}
			src.Close()
		}
	}

	// Add manifest.json at root
	if release.ManifestJSON != "" {
		fw, err := zw.Create("manifest.json")
		if err == nil {
			if _, err := fw.Write([]byte(release.ManifestJSON)); err != nil {
				log.Printf("[package] write manifest error: %v", err)
			}
		}
		// Also write manifest.json to disk alongside the zip
		manifestPath := filepath.Join(packageDir, zipName[:len(zipName)-4]+"_manifest.json")
		if err := os.WriteFile(manifestPath, []byte(release.ManifestJSON), 0644); err != nil {
			log.Printf("[package] write manifest to disk error: %v", err)
		} else {
			log.Printf("[package] manifest written to: %s", manifestPath)
		}
	}

	log.Printf("[package] created package: %s (%d artifacts from %d builds)", zipPath, len(allArtifacts), len(matchingBuilds))
	return zipPath, nil
}

// extractArtifactToZip tries to extract a zip artifact's contents into targetDir within the output zip.
// Returns true if the artifact was a zip and was extracted, false otherwise.
func (s *PackageService) extractArtifactToZip(zw *zip.Writer, artPath, fileName, targetDir string, addedFiles map[string]bool) (bool, error) {
	// Only attempt extraction for zip files
	ext := strings.ToLower(filepath.Ext(fileName))
	if ext != ".zip" && ext != ".jar" && ext != ".war" {
		return false, nil
	}

	srcReader, err := zip.OpenReader(artPath)
	if err != nil {
		return false, err
	}
	defer srcReader.Close()

	for _, f := range srcReader.File {
		entryName := fmt.Sprintf("%s/%s", targetDir, f.Name)
		if addedFiles[entryName] {
			continue
		}
		addedFiles[entryName] = true

		if f.FileInfo().IsDir() {
			if _, err := zw.Create(entryName); err != nil {
				log.Printf("[package] create dir %s error: %v", entryName, err)
			}
			continue
		}

		fw, err := zw.Create(entryName)
		if err != nil {
			log.Printf("[package] create entry %s error: %v", entryName, err)
			continue
		}

		rc, err := f.Open()
		if err != nil {
			log.Printf("[package] open zip entry %s error: %v", f.Name, err)
			continue
		}
		if _, err := io.Copy(fw, rc); err != nil {
			log.Printf("[package] copy zip entry %s error: %v", f.Name, err)
		}
		rc.Close()
	}

	return true, nil
}

// CheckReleaseComplete checks if all builds for a release are complete (success or failed).
// Returns (allDone bool, allSuccess bool).
func (s *PackageService) CheckReleaseComplete(releaseID uint) (bool, bool) {
	releaseBuilds, err := s.buildStore.ListByReleaseID(releaseID)
	if err != nil || len(releaseBuilds) == 0 {
		return false, false
	}

	allDone := true
	allSuccess := true
	for _, b := range releaseBuilds {
		if b.Status == model.BuildStatusPending || b.Status == model.BuildStatusRunning {
			allDone = false
			break
		}
		if b.Status != model.BuildStatusSuccess {
			allSuccess = false
		}
	}

	return allDone, allSuccess
}

// GetReleaseBuilds returns all builds for a specific release
func (s *PackageService) GetReleaseBuilds(releaseID uint) ([]model.Build, error) {
	return s.buildStore.ListByReleaseID(releaseID)
}
