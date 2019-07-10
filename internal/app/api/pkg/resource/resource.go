package resource

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/thecodingmachine/gotenberg/internal/pkg/config"
	"github.com/thecodingmachine/gotenberg/internal/pkg/logger"
	"github.com/thecodingmachine/gotenberg/internal/pkg/printer"
	"github.com/thecodingmachine/gotenberg/internal/pkg/standarderror"
)

const (
	// ResultFilenameFormField contains the name
	// of a form field.
	ResultFilenameFormField string = "resultFilename"
	// WaitTimeoutFormField contains the name
	// of a form field.
	WaitTimeoutFormField string = "waitTimeout"
	// WebhookURLFormField contains the name
	// of a form field.
	WebhookURLFormField string = "webhookURL"
	// RemoteURLFormField contains the name
	// of a form field.
	RemoteURLFormField string = "remoteURL"
	// WaitDelayFormField contains the name
	// of a form field.
	WaitDelayFormField string = "waitDelay"
	// PaperWidthFormField contains the name
	// of a form field.
	PaperWidthFormField string = "paperWidth"
	// PaperHeightFormField contains the name
	// of a form field.
	PaperHeightFormField string = "paperHeight"
	// MarginTopFormField contains the name
	// of a form field.
	MarginTopFormField string = "marginTop"
	// MarginBottomFormField contains the name
	// of a form field.
	MarginBottomFormField string = "marginBottom"
	// MarginLeftFormField contains the name
	// of a form field.
	MarginLeftFormField string = "marginLeft"
	// MarginRightFormField contains the name
	// of a form field.
	MarginRightFormField string = "marginRight"
	// LandscapeFormField contains the name
	// of a form field.
	LandscapeFormField string = "landscape"
)

// Resource helps retrieving form values
// and form files from a request.
type Resource struct {
	logger           *logger.Logger
	config           *config.Config
	formValues       map[string]string
	formFilesDirPath string
}

// New creates a new resource.
func New(c echo.Context, logger *logger.Logger, config *config.Config, dirPath string) (*Resource, error) {
	const op string = "resource.New"
	r := &Resource{
		logger:           logger,
		config:           config,
		formValues:       formValues(c, logger),
		formFilesDirPath: dirPath,
	}
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil, &standarderror.Error{Op: op, Err: err}
	}
	r.logger.DebugfOp(op, "directory '%s' created", dirPath)
	if err := formFiles(c, logger, dirPath); err != nil {
		return r, &standarderror.Error{Op: op, Err: err}
	}
	return r, nil
}

func formValues(c echo.Context, logger *logger.Logger) map[string]string {
	const op string = "resource.formValues"
	v := make(map[string]string)
	fetch := func(formField string) string {
		value := c.FormValue(formField)
		if value == "" {
			logger.DebugfOp(op, "'%s' is empty", formField)
			return value
		}
		logger.DebugfOp(op, "'%s' retrieved, got '%s'", formField, value)
		return value
	}
	v[ResultFilenameFormField] = fetch(ResultFilenameFormField)
	v[WaitTimeoutFormField] = fetch(WaitTimeoutFormField)
	v[WebhookURLFormField] = fetch(WebhookURLFormField)
	v[RemoteURLFormField] = fetch(RemoteURLFormField)
	v[WaitDelayFormField] = fetch(WaitDelayFormField)
	v[PaperWidthFormField] = fetch(PaperWidthFormField)
	v[PaperHeightFormField] = fetch(PaperHeightFormField)
	v[MarginTopFormField] = fetch(MarginTopFormField)
	v[MarginBottomFormField] = fetch(MarginBottomFormField)
	v[MarginLeftFormField] = fetch(MarginLeftFormField)
	v[MarginRightFormField] = fetch(MarginRightFormField)
	v[LandscapeFormField] = fetch(LandscapeFormField)
	return v
}

func formFiles(c echo.Context, logger *logger.Logger, dirPath string) error {
	const op string = "resource.formFiles"
	form, err := c.MultipartForm()
	if err != nil {
		return &standarderror.Error{Op: op, Err: err}
	}
	for _, files := range form.File {
		for _, fh := range files {
			in, err := fh.Open()
			if err != nil {
				return &standarderror.Error{Op: op, Err: err}
			}
			defer in.Close() // nolint: errcheck
			fpath := fmt.Sprintf("%s/%s", dirPath, fh.Filename)
			out, err := os.Create(fpath)
			if err != nil {
				return &standarderror.Error{Op: op, Err: err}
			}
			defer out.Close() // nolint: errcheck
			if err := out.Chmod(0644); err != nil {
				return &standarderror.Error{Op: op, Err: err}
			}
			if _, err := io.Copy(out, in); err != nil {
				return &standarderror.Error{Op: op, Err: err}
			}
			if _, err := out.Seek(0, 0); err != nil {
				return &standarderror.Error{Op: op, Err: err}
			}
			logger.DebugfOp(op, "'%s' created", fh.Filename)
		}
	}
	return nil
}

// DirPath returns the directory
// path where are stored the form
// files and the resulting PDF file.
func (r *Resource) DirPath() string {
	return r.formFilesDirPath
}

// Close deletes the working directory of the
// resource if it exists.
func (r *Resource) Close() error {
	const op string = "resource.Close"
	if _, err := os.Stat(r.formFilesDirPath); os.IsNotExist(err) {
		r.logger.DebugfOp(op, "directory '%s' does not exist, nothing to remove", r.formFilesDirPath)
		return nil
	}
	if err := os.RemoveAll(r.formFilesDirPath); err != nil {
		return &standarderror.Error{Op: op, Err: err}
	}
	r.logger.DebugfOp(op, "directory '%s' removed", r.formFilesDirPath)
	return nil
}

const defaultHeaderFooterHTML string = "<html><head></head><body></body></html>"

// ChromePrinterOptions returns the Chrome printer options
// thanks to the form values and form files from the request
// plus the default values from the configuration.
func (r *Resource) ChromePrinterOptions() (*printer.ChromeOptions, error) {
	const op string = "resource.ChromePrinterOptions"
	waitTimeout, err := r.float64(WaitTimeoutFormField, r.config.DefaultWaitTimeout())
	if err != nil {
		return nil, &standarderror.Error{Op: op, Err: err}
	}
	waitDelay, err := r.float64(WaitDelayFormField, 0.0)
	if err != nil {
		return nil, &standarderror.Error{Op: op, Err: err}
	}
	headerHTML, err := r.content("header.html", defaultHeaderFooterHTML)
	if err != nil {
		return nil, &standarderror.Error{Op: op, Err: err}
	}
	footerHTML, err := r.content("footer.html", defaultHeaderFooterHTML)
	if err != nil {
		return nil, &standarderror.Error{Op: op, Err: err}
	}
	paperWidth, err := r.float64(PaperWidthFormField, 8.27)
	if err != nil {
		return nil, &standarderror.Error{Op: op, Err: err}
	}
	paperHeight, err := r.float64(PaperHeightFormField, 11.7)
	if err != nil {
		return nil, &standarderror.Error{Op: op, Err: err}
	}
	marginTop, err := r.float64(MarginTopFormField, 1)
	if err != nil {
		return nil, &standarderror.Error{Op: op, Err: err}
	}
	marginBottom, err := r.float64(MarginBottomFormField, 1)
	if err != nil {
		return nil, &standarderror.Error{Op: op, Err: err}
	}
	marginLeft, err := r.float64(MarginLeftFormField, 1)
	if err != nil {
		return nil, &standarderror.Error{Op: op, Err: err}
	}
	marginRight, err := r.float64(MarginRightFormField, 1)
	if err != nil {
		return nil, &standarderror.Error{Op: op, Err: err}
	}
	landscape, err := r.bool(LandscapeFormField, false)
	if err != nil {
		return nil, &standarderror.Error{Op: op, Err: err}
	}
	opts := &printer.ChromeOptions{
		WaitTimeout:  waitTimeout,
		WaitDelay:    waitDelay,
		HeaderHTML:   headerHTML,
		FooterHTML:   footerHTML,
		PaperWidth:   paperWidth,
		PaperHeight:  paperHeight,
		MarginTop:    marginTop,
		MarginBottom: marginBottom,
		MarginLeft:   marginLeft,
		MarginRight:  marginRight,
		Landscape:    landscape,
	}
	r.logger.DebugfOp(op, "printer options: %+v", opts)
	return opts, nil
}

// OfficePrinterOptions returns the Office printer options
// thanks to the form values from the request
// plus the default values from the configuration.
func (r *Resource) OfficePrinterOptions() (*printer.OfficeOptions, error) {
	const op string = "resource.OfficePrinterOptions"
	waitTimeout, err := r.float64(WaitTimeoutFormField, r.config.DefaultWaitTimeout())
	if err != nil {
		return nil, &standarderror.Error{Op: op, Err: err}
	}
	landscape, err := r.bool(LandscapeFormField, false)
	if err != nil {
		return nil, &standarderror.Error{Op: op, Err: err}
	}
	opts := &printer.OfficeOptions{
		WaitTimeout: waitTimeout,
		Landscape:   landscape,
	}
	r.logger.DebugfOp(op, "printer options: %+v", opts)
	return opts, nil
}

// MergePrinterOptions returns the merge printer options
// thanks to the form values from the request
// plus the default values from the configuration.
func (r *Resource) MergePrinterOptions() (*printer.MergeOptions, error) {
	const op string = "resource.MergePrinterOptions"
	waitTimeout, err := r.float64(WaitTimeoutFormField, r.config.DefaultWaitTimeout())
	if err != nil {
		return nil, &standarderror.Error{Op: op, Err: err}
	}
	opts := &printer.MergeOptions{
		WaitTimeout: waitTimeout,
	}
	r.logger.DebugfOp(op, "printer options: %+v", opts)
	return opts, nil
}

// Has returns true if the resource
// contains the given form field and
// its value is not empty.
func (r *Resource) Has(formField string) bool {
	v, ok := r.formValues[formField]
	if ok {
		ok = v != ""
	}
	return ok
}

func (r *Resource) hasFile(filename string) bool {
	fpath := fmt.Sprintf("%s/%s", r.formFilesDirPath, filename)
	_, err := os.Stat(fpath)
	return !os.IsNotExist(err)
}

// Get returns the form field value.
func (r *Resource) Get(formField string) (string, error) {
	const op string = "resource.Get"
	v, err := r.value(formField)
	if err != nil {
		return "", &standarderror.Error{Op: op, Err: err}
	}
	return v, nil
}

func (r *Resource) value(formField string) (string, error) {
	const op string = "resource.value"
	v, ok := r.formValues[formField]
	if !ok {
		return "", &standarderror.Error{
			Code:    standarderror.Invalid,
			Message: fmt.Sprintf("'%s' does not exist", formField),
			Op:      op,
		}
	}
	return v, nil
}

func (r *Resource) float64(formField string, defaultValue float64) (float64, error) {
	const op string = "resource.float64"
	if !r.Has(formField) {
		return defaultValue, nil
	}
	v, err := r.value(formField)
	if err != nil {
		return 0.0, &standarderror.Error{Op: op, Err: err}
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0.0, &standarderror.Error{
			Code:    standarderror.Invalid,
			Message: fmt.Sprintf("'%s' is not a float, got '%s'", formField, v),
			Op:      op,
		}
	}
	return f, nil
}

func (r *Resource) bool(formField string, defaultValue bool) (bool, error) {
	const op string = "resource.bool"
	if !r.Has(formField) {
		return defaultValue, nil
	}
	v, err := r.value(formField)
	if err != nil {
		return false, &standarderror.Error{Op: op, Err: err}
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, &standarderror.Error{
			Code:    standarderror.Invalid,
			Message: fmt.Sprintf("'%s' is not a boolean, got '%s'", formField, v),
			Op:      op,
		}
	}
	return b, nil
}

// Fpath returns the path of the given filename.
// This filename should be the name of a form file.
func (r *Resource) Fpath(filename string) (string, error) {
	const op string = "resource.Fpath"
	fpath := fmt.Sprintf("%s/%s", r.formFilesDirPath, filename)
	_, err := os.Stat(fpath)
	if os.IsNotExist(err) {
		return "", &standarderror.Error{
			Code:    standarderror.Invalid,
			Message: fmt.Sprintf("file '%s' does not exist", filename),
			Op:      op,
		}
	}
	absPath, err := filepath.Abs(fpath)
	if err != nil {
		return "", &standarderror.Error{Op: op, Err: err}
	}
	return absPath, nil
}

func (r *Resource) content(filename string, defaultValue string) (string, error) {
	const op string = "resource.content"
	if !r.hasFile(filename) {
		return defaultValue, nil
	}
	fpath, err := r.Fpath(filename)
	if err != nil {
		return "", &standarderror.Error{Op: op, Err: err}
	}
	b, err := ioutil.ReadFile(fpath)
	if err != nil {
		return "", &standarderror.Error{Op: op, Err: err}
	}
	return string(b), nil
}

// Fpaths returns the list of files of the resource
// according to given file extensions.
func (r *Resource) Fpaths(exts ...string) ([]string, error) {
	const op string = "resource.Fpaths"
	var fpaths []string
	err := filepath.Walk(r.formFilesDirPath, func(path string, info os.FileInfo, _ error) error {
		if info.IsDir() {
			return nil
		}
		fpath, err := r.Fpath(info.Name())
		if err != nil {
			return &standarderror.Error{Op: op, Err: err}
		}
		for _, ext := range exts {
			if filepath.Ext(fpath) == ext {
				fpaths = append(fpaths, fpath)
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return nil, &standarderror.Error{Op: op, Err: err}
	}
	if len(fpaths) == 0 {
		return nil, &standarderror.Error{
			Code:    standarderror.Invalid,
			Message: fmt.Sprintf("no file found for extentions %v", exts),
			Op:      op,
		}
	}
	return fpaths, nil
}
