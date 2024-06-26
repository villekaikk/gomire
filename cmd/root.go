package cmd

import (
	"fmt"
	"gomire/internal/utils"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/disintegration/imaging"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

var (
	inputDir          string
	outputDir         string
	recursive         bool
	verbose           bool
	fileType          string
	fileTypes         []string
	resolution        string
	supportedImgTypes = []string{"jpg", "png", "gif", "tif", "bmp"}
)

type FileOperation struct {
	originPath   string
	targetPath   string
	targetWidth  int
	targetHeight int
}

func NewFileOperation(o_path string, t_path string, res string) *FileOperation {

	splits := strings.Split(res, "x")
	w, err := strconv.Atoi(splits[0])

	if err != nil {
		fmt.Printf("Unable to parse width from %s: %e", res, err)
		os.Exit(2)
	}

	h, err := strconv.Atoi(splits[1])

	if err != nil {
		fmt.Printf("Unable to parse height from %s: %e", res, err)
		os.Exit(2)
	}

	return &FileOperation{
		originPath:   o_path,
		targetPath:   t_path,
		targetWidth:  w,
		targetHeight: h,
	}
}

var rootCmd = &cobra.Command{

	Use:   "gomire",
	Short: "Tool for resizing images en masse",
	Long:  "",
	Run:   cmdMain,
}

func Execute() {

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {

	rootCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Find and resize images from subfolders (required)")
	rootCmd.Flags().StringVarP(&inputDir, "input-dir", "i", "", "Location to the image directory (required)")
	rootCmd.Flags().StringVarP(&outputDir, "output-dir", "o", "", "Location to the output directory. Will be created if does not exist (required)")
	rootCmd.Flags().StringVarP(&fileType, "type", "t", "png,jpg", fmt.Sprintf("Image file type(s) separated by commas. Supported filetypes are %s", strings.Join(supportedImgTypes, ", ")))
	rootCmd.Flags().StringVarP(&resolution, "resolution", "R", "", "Target image resolution in <width>x<height> format (e.g 1920x1080) (required)")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Print reasoning for all failed operations")

	rootCmd.MarkFlagRequired("input-dir")
	rootCmd.MarkFlagRequired("output-dir")
	rootCmd.MarkFlagRequired("resolution")
}

// Main function for the tool
func cmdMain(cmd *cobra.Command, args []string) {

	validateFlags()
	files, err := listFilesToBeCopies()

	if err != nil {
		fmt.Printf("Error enumerating input files: %e\n", err)
		os.Exit(2)
	}

	if len(files) == 0 {
		fmt.Println("No files found")
		os.Exit(0)
	}

	copyFilesWithProgress(files)
}

func validateFlags() {

	// Input dir
	var err error
	inputDir, err := filepath.Abs(inputDir)
	if err != nil {
		fmt.Printf("Error validating input directory path %s: %e\n", inputDir, err)
		os.Exit(2)
	}

	e, _ := utils.PathExists(inputDir)
	if !e {
		fmt.Printf("Error: Input directory %s does not exist\n", inputDir)
		os.Exit(2)
	}

	// Output dir
	outputDir, err = filepath.Abs(outputDir)
	if err != nil {
		fmt.Printf("Error validating input directory path %s: %e\n", outputDir, err)
		os.Exit(2)
	}

	e, _ = utils.PathExists(outputDir)
	if !e {
		err := os.MkdirAll(outputDir, 0755)
		if err != nil {
			fmt.Printf("Error creaging output directory %s: %e\n", outputDir, err)
		}
	}

	// If output dir exists within input dir
	if strings.Contains(outputDir, inputDir) {
		fmt.Println("Error: Output directory can't be a sub directory of the input directory")
		os.Exit(2)
	}

	fileType = strings.Replace(fileType, "jpeg", "jpg", -1)
	fileType = strings.Replace(fileType, "tiff", "tif", -1)

	// Format all requested filetypes to be like ".jpg"
	fileTypes = strings.Split(fileType, ",")
	for i, v := range fileTypes {

		if utils.IsStringEmpty(v) {
			continue
		}

		if v[0] != '.' {
			fileTypes[i] = fmt.Sprintf(".%s", v)
		}
	}

	// Resolution
	resolution = strings.ToLower(resolution)
	resolution = strings.TrimSpace(resolution)
	if !strings.Contains(resolution, "x") {
		fmt.Println("Invalid resolution format: missing \"x\" as a dimensional separator")
		os.Exit(2)
	}

}

func listFilesToBeCopies() ([]FileOperation, error) {

	var files []FileOperation

	filepath.WalkDir(inputDir, func(s string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}

		if d.IsDir() {
			// Ignore subdirectories if not run as recursive
			if s != inputDir && !recursive {
				// fmt.Printf("\"%s\" is a not root \"%s\" -> skipping\n", s, inputDir)
				return fs.SkipDir
			}

			// Ignore plain directory paths
			return nil
		}

		if isSupportedFiletype(filepath.Ext(s)) {
			t := strings.Replace(s, inputDir, outputDir, 1)
			files = append(files, *NewFileOperation(s, t, resolution))
		}

		return nil
	})

	return files, nil
}

func isSupportedFiletype(s string) bool {

	return slices.Contains(supportedImgTypes, strings.ToLower(s)[1:])
}

// Loops through all requested files and displays CLI progress bar during the operation
func copyFilesWithProgress(fo []FileOperation) {

	bar := progressbar.NewOptions(
		len(fo),
		progressbar.OptionSetDescription("Resizing images:"),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(-1),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionThrottle(time.Duration(200*time.Millisecond)),
	)

	time.Sleep(500 * time.Millisecond)
	var errs []string
	var wg sync.WaitGroup
	rand.NewSource(time.Now().UnixNano())

	for _, f := range fo {
		wg.Add(1)

		go func(fo FileOperation) {
			// HOX: We can't print out anything before this loop is finished as it messes up the progress bar
			defer wg.Done()
			defer bar.Add(1)
			err := processImage(fo)
			if err != nil {
				errs = append(errs, fmt.Sprintf("Error processing image: %s\n", err.Error()))
			}
		}(f)
	}

	wg.Wait()

	// Print out errors occured during copying, after the progressbar has finished
	if len(errs) > 0 {
		fmt.Printf("\n%d image operations failed\n", len(errs))
	}
	if verbose {
		for _, e := range errs {
			fmt.Println(e)
		}
	}
}

func processImage(fo FileOperation) error {

	// fmt.Printf("Processing image %s\n", fo.originPath)

	srcImg, err := imaging.Open(fo.originPath)
	if err != nil {
		return fmt.Errorf("error opening image %s: %s", fo.originPath, err.Error())
	}

	dstImg := imaging.Resize(srcImg, fo.targetWidth, fo.targetHeight, imaging.Lanczos)

	err = os.MkdirAll(filepath.Dir(fo.targetPath), os.ModeDir)
	if err != nil {
		return fmt.Errorf("error creating destination folder %s: %s", fo.targetPath, err.Error())
	}

	err = imaging.Save(dstImg, fo.targetPath)
	if err != nil {
		return fmt.Errorf("error saving resized image %s: %s", fo.targetPath, err.Error())
	}

	return nil
}
