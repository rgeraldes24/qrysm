package main

import (
	"slices"

	"github.com/theQRL/qrysm/tools/analyzers/comparesame"
	"github.com/theQRL/qrysm/tools/analyzers/cryptorand"
	"github.com/theQRL/qrysm/tools/analyzers/errcheck"
	"github.com/theQRL/qrysm/tools/analyzers/featureconfig"
	"github.com/theQRL/qrysm/tools/analyzers/gocognit"
	"github.com/theQRL/qrysm/tools/analyzers/httpwriter"
	"github.com/theQRL/qrysm/tools/analyzers/ineffassign"
	"github.com/theQRL/qrysm/tools/analyzers/interfacechecker"
	"github.com/theQRL/qrysm/tools/analyzers/logcapitalization"
	"github.com/theQRL/qrysm/tools/analyzers/logruswitherror"
	"github.com/theQRL/qrysm/tools/analyzers/maligned"
	modernizeany "github.com/theQRL/qrysm/tools/analyzers/modernize/any"
	"github.com/theQRL/qrysm/tools/analyzers/modernize/appendclipped"
	"github.com/theQRL/qrysm/tools/analyzers/modernize/bloop"
	"github.com/theQRL/qrysm/tools/analyzers/modernize/fmtappendf"
	"github.com/theQRL/qrysm/tools/analyzers/modernize/forvar"
	"github.com/theQRL/qrysm/tools/analyzers/modernize/mapsloop"
	"github.com/theQRL/qrysm/tools/analyzers/modernize/minmax"
	"github.com/theQRL/qrysm/tools/analyzers/modernize/omitzero"
	"github.com/theQRL/qrysm/tools/analyzers/modernize/rangeint"
	"github.com/theQRL/qrysm/tools/analyzers/modernize/reflecttypefor"
	"github.com/theQRL/qrysm/tools/analyzers/modernize/slicescontains"
	"github.com/theQRL/qrysm/tools/analyzers/modernize/slicessort"
	"github.com/theQRL/qrysm/tools/analyzers/modernize/stringsbuilder"
	"github.com/theQRL/qrysm/tools/analyzers/modernize/stringscutprefix"
	"github.com/theQRL/qrysm/tools/analyzers/modernize/stringsseq"
	"github.com/theQRL/qrysm/tools/analyzers/modernize/testingcontext"
	modernizewaitgroup "github.com/theQRL/qrysm/tools/analyzers/modernize/waitgroup"
	"github.com/theQRL/qrysm/tools/analyzers/nop"
	"github.com/theQRL/qrysm/tools/analyzers/nopanic"
	"github.com/theQRL/qrysm/tools/analyzers/properpermissions"
	"github.com/theQRL/qrysm/tools/analyzers/recursivelock"
	"github.com/theQRL/qrysm/tools/analyzers/shadowpredecl"
	"github.com/theQRL/qrysm/tools/analyzers/slicedirect"
	"github.com/theQRL/qrysm/tools/analyzers/uintcast"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/appends"
	"golang.org/x/tools/go/analysis/passes/asmdecl"
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/atomicalign"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/analysis/passes/buildtag"
	"golang.org/x/tools/go/analysis/passes/composite"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/analysis/passes/deepequalerrors"
	"golang.org/x/tools/go/analysis/passes/defers"
	"golang.org/x/tools/go/analysis/passes/directive"
	"golang.org/x/tools/go/analysis/passes/errorsas"
	"golang.org/x/tools/go/analysis/passes/findcall"
	"golang.org/x/tools/go/analysis/passes/framepointer"
	"golang.org/x/tools/go/analysis/passes/httpmux"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/ifaceassert"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/analysis/passes/loopclosure"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/nilness"
	"golang.org/x/tools/go/analysis/passes/pkgfact"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/reflectvaluecompare"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/sigchanyzer"
	"golang.org/x/tools/go/analysis/passes/slog"
	"golang.org/x/tools/go/analysis/passes/sortslice"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/stringintconv"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/testinggoroutine"
	"golang.org/x/tools/go/analysis/passes/tests"
	"golang.org/x/tools/go/analysis/passes/timeformat"
	"golang.org/x/tools/go/analysis/passes/unmarshal"
	"golang.org/x/tools/go/analysis/passes/unreachable"
	"golang.org/x/tools/go/analysis/passes/unsafeptr"
	"golang.org/x/tools/go/analysis/passes/unusedresult"
	"golang.org/x/tools/go/analysis/passes/unusedwrite"
	"golang.org/x/tools/go/analysis/passes/usesgenerics"
)

// Disabled on purpose:
//   - modernize/newexpr
//   - modernize/slicesdelete: see https://go.dev/issue/73686
//   - cgocall, fieldalignment, shadow
var qrysmAnalyzers = []*analysis.Analyzer{
	comparesame.Analyzer,
	cryptorand.Analyzer,
	errcheck.Analyzer,
	featureconfig.Analyzer,
	gocognit.Analyzer,
	httpwriter.Analyzer,
	ineffassign.Analyzer,
	interfacechecker.Analyzer,
	logcapitalization.Analyzer,
	logruswitherror.Analyzer,
	maligned.Analyzer,
	modernizeany.Analyzer,
	appendclipped.Analyzer,
	bloop.Analyzer,
	fmtappendf.Analyzer,
	forvar.Analyzer,
	mapsloop.Analyzer,
	minmax.Analyzer,
	omitzero.Analyzer,
	rangeint.Analyzer,
	reflecttypefor.Analyzer,
	slicescontains.Analyzer,
	slicessort.Analyzer,
	stringsbuilder.Analyzer,
	stringscutprefix.Analyzer,
	stringsseq.Analyzer,
	testingcontext.Analyzer,
	modernizewaitgroup.Analyzer,
	nop.Analyzer,
	nopanic.Analyzer,
	properpermissions.Analyzer,
	recursivelock.Analyzer,
	shadowpredecl.Analyzer,
	slicedirect.Analyzer,
	uintcast.Analyzer,
}

var goAnalyzers = []*analysis.Analyzer{
	appends.Analyzer,
	asmdecl.Analyzer,
	assign.Analyzer,
	atomic.Analyzer,
	atomicalign.Analyzer,
	bools.Analyzer,
	buildssa.Analyzer,
	buildtag.Analyzer,
	composite.Analyzer,
	copylock.Analyzer,
	ctrlflow.Analyzer,
	deepequalerrors.Analyzer,
	defers.Analyzer,
	directive.Analyzer,
	errorsas.Analyzer,
	findcall.Analyzer,
	framepointer.Analyzer,
	httpmux.Analyzer,
	httpresponse.Analyzer,
	ifaceassert.Analyzer,
	inspect.Analyzer,
	loopclosure.Analyzer,
	lostcancel.Analyzer,
	nilfunc.Analyzer,
	nilness.Analyzer,
	pkgfact.Analyzer,
	printf.Analyzer,
	reflectvaluecompare.Analyzer,
	shift.Analyzer,
	sigchanyzer.Analyzer,
	slog.Analyzer,
	sortslice.Analyzer,
	stdmethods.Analyzer,
	stringintconv.Analyzer,
	structtag.Analyzer,
	testinggoroutine.Analyzer,
	tests.Analyzer,
	timeformat.Analyzer,
	unmarshal.Analyzer,
	unreachable.Analyzer,
	unsafeptr.Analyzer,
	unusedresult.Analyzer,
	unusedwrite.Analyzer,
	usesgenerics.Analyzer,
}

// staticcheckChecks are staticcheck checks. qrysm-vet does not run staticcheck: the list is only
// used to exclude the generated files from these checks (see loadConfigs). Keep it sorted.
var staticcheckChecks = []string{
	"sa1000", "sa1001", "sa1002", "sa1003", "sa1004", "sa1005", "sa1006", "sa1007", "sa1008",
	"sa1010", "sa1011", "sa1012", "sa1013", "sa1014", "sa1015", "sa1016", "sa1017", "sa1018",
	// "sa1019", // TODO: Fix all uses of deprecated things.
	"sa1020", "sa1021", "sa1023", "sa1024", "sa1025", "sa1026", "sa1027", "sa1028", "sa1029",
	"sa1030",
	"sa2000", "sa2001", "sa2002", "sa2003",
	"sa3000", "sa3001",
	"sa4000", "sa4001", "sa4003", "sa4004", "sa4005", "sa4006", "sa4008", "sa4009", "sa4010",
	"sa4011", "sa4012", "sa4013", "sa4014", "sa4015", "sa4016", "sa4017", "sa4018", "sa4019",
	"sa4020", "sa4021", "sa4022", "sa4023", "sa4024", "sa4025", "sa4026", "sa4027", "sa4028",
	"sa4029", "sa4030", "sa4031", "sa4032",
	"sa5000", "sa5001", "sa5002", "sa5003", "sa5004", "sa5005", "sa5007", "sa5008", "sa5009",
	"sa5010", "sa5011", "sa5012",
	"sa6000", "sa6001", "sa6002", "sa6003", "sa6005", "sa6006",
	"sa9001", "sa9002", "sa9003", "sa9004", "sa9005", "sa9006", "sa9007", "sa9008",
}

// allAnalyzers returns every analyzer qrysm-vet runs, before applying the config.
func allAnalyzers() []*analysis.Analyzer {
	return slices.Concat(qrysmAnalyzers, goAnalyzers)
}
