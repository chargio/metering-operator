package reporting

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	metering "github.com/operator-framework/operator-metering/pkg/apis/metering/v1alpha1"
	meteringClient "github.com/operator-framework/operator-metering/pkg/generated/clientset/versioned/typed/metering/v1alpha1"
	meteringListers "github.com/operator-framework/operator-metering/pkg/generated/listers/metering/v1alpha1"
)

const maxDepth = 100

type ReportGenerationQueryDependencies struct {
	ReportGenerationQueries        []*metering.ReportGenerationQuery
	DynamicReportGenerationQueries []*metering.ReportGenerationQuery
	ReportDataSources              []*metering.ReportDataSource
	Reports                        []*metering.Report
	ScheduledReports               []*metering.ScheduledReport
}

func ValidateGenerationQueryDependenciesStatus(depsStatus *GenerationQueryDependenciesStatus) (*ReportGenerationQueryDependencies, error) {
	// if the specified ReportGenerationQuery depends on other non-dynamic
	// ReportGenerationQueries, but they have their view disabled, then it's an
	// invalid configuration.
	var (
		queriesViewDisabled,
		uninitializedQueries,
		uninitializedDataSources,
		uninitializedReports,
		uninitializedScheduledReports []string
	)

	for _, query := range depsStatus.UninitializedReportGenerationQueries {
		if query.Spec.View.Disabled {
			queriesViewDisabled = append(queriesViewDisabled, query.Name)
		} else if query.ViewName == "" {
			uninitializedQueries = append(uninitializedQueries, query.Name)
		}
	}
	for _, ds := range depsStatus.UninitializedReportDataSources {
		uninitializedDataSources = append(uninitializedDataSources, ds.Name)
	}
	for _, report := range depsStatus.UninitializedReports {
		uninitializedReports = append(uninitializedReports, report.Name)
	}
	for _, scheduledReport := range depsStatus.UninitializedScheduledReports {
		uninitializedScheduledReports = append(uninitializedScheduledReports, scheduledReport.Name)
	}

	var errs []string
	if len(queriesViewDisabled) != 0 {
		errs = append(errs, fmt.Sprintf("invalid ReportGenerationQuery, references ReportGenerationQueries with spec.view.disabled=true: %s", strings.Join(queriesViewDisabled, ", ")))
	}
	if len(uninitializedDataSources) != 0 {
		errs = append(errs, fmt.Sprintf("ReportGenerationQuery has uninitialized ReportDataSource dependencies: %s", strings.Join(uninitializedDataSources, ", ")))
	}
	if len(uninitializedQueries) != 0 {
		errs = append(errs, fmt.Sprintf("ReportGenerationQuery has uninitialized ReportGenerationQuery dependencies: %s", strings.Join(uninitializedQueries, ", ")))
	}
	if len(uninitializedReports) != 0 {
		errs = append(errs, fmt.Sprintf("ReportGenerationQuery has uninitialized Report dependencies: %s", strings.Join(uninitializedReports, ", ")))
	}
	if len(uninitializedScheduledReports) != 0 {
		errs = append(errs, fmt.Sprintf("ReportGenerationQuery has uninitialized ScheduledReport dependencies: %s", strings.Join(uninitializedScheduledReports, ", ")))
	}

	if len(errs) != 0 {
		return nil, fmt.Errorf("ReportGenerationQuery dependency validation error: %s", strings.Join(errs, ", "))
	}

	return &ReportGenerationQueryDependencies{
		ReportGenerationQueries:        depsStatus.InitializedReportGenerationQueries,
		DynamicReportGenerationQueries: depsStatus.InitializedDynamicReportGenerationQueries,
		ReportDataSources:              depsStatus.InitializedReportDataSources,
		Reports:                        depsStatus.InitializedReports,
		ScheduledReports:               depsStatus.InitializedScheduledReports,
	}, nil
}

type GenerationQueryDependenciesStatus struct {
	UninitializedReportGenerationQueries      []*metering.ReportGenerationQuery
	InitializedReportGenerationQueries        []*metering.ReportGenerationQuery
	InitializedDynamicReportGenerationQueries []*metering.ReportGenerationQuery

	UninitializedReports []*metering.Report
	InitializedReports   []*metering.Report

	UninitializedScheduledReports []*metering.ScheduledReport
	InitializedScheduledReports   []*metering.ScheduledReport

	UninitializedReportDataSources []*metering.ReportDataSource
	InitializedReportDataSources   []*metering.ReportDataSource
}

func GetGenerationQueryDependenciesStatus(
	queryGetter reportGenerationQueryGetter,
	dataSourceGetter reportDataSourceGetter,
	reportGetter reportGetter,
	scheduledReportGetter scheduledReportGetter,
	generationQuery *metering.ReportGenerationQuery,
) (*GenerationQueryDependenciesStatus, error) {
	// Validate ReportGenerationQuery's that should be views
	dependentQueriesStatus, err := GetDependentGenerationQueries(queryGetter, generationQuery)
	if err != nil {
		return nil, err
	}

	dataSources, err := GetDependentDataSources(dataSourceGetter, generationQuery)
	if err != nil {
		return nil, err
	}

	reports, err := GetDependentReports(reportGetter, generationQuery)
	if err != nil {
		return nil, err
	}

	scheduledReports, err := GetDependentScheduledReports(scheduledReportGetter, generationQuery)
	if err != nil {
		return nil, err
	}

	var uninitializedDataSources, initializedDataSources []*metering.ReportDataSource
	for _, dataSource := range dataSources {
		if dataSource.TableName == "" {
			uninitializedDataSources = append(uninitializedDataSources, dataSource)
		} else {
			initializedDataSources = append(initializedDataSources, dataSource)
		}
	}

	var uninitializedQueries, initializedQueries []*metering.ReportGenerationQuery
	for _, query := range dependentQueriesStatus.ViewReportGenerationQueries {
		if query.ViewName == "" {
			uninitializedQueries = append(uninitializedQueries, query)
		} else {
			initializedQueries = append(initializedQueries, query)
		}
	}

	var uninitializedReports, initializedReports []*metering.Report
	for _, report := range reports {
		if report.Status.TableName == "" {
			uninitializedReports = append(uninitializedReports, report)
		} else {
			initializedReports = append(initializedReports, report)
		}
	}

	var uninitializedScheduledReports, initializedScheduledReports []*metering.ScheduledReport
	for _, scheduledReport := range scheduledReports {
		if scheduledReport.Status.TableName == "" {
			uninitializedScheduledReports = append(uninitializedScheduledReports, scheduledReport)
		} else {
			initializedScheduledReports = append(initializedScheduledReports, scheduledReport)
		}
	}

	return &GenerationQueryDependenciesStatus{
		UninitializedReportGenerationQueries:      uninitializedQueries,
		InitializedReportGenerationQueries:        initializedQueries,
		InitializedDynamicReportGenerationQueries: dependentQueriesStatus.DynamicReportGenerationQueries,
		UninitializedReportDataSources:            uninitializedDataSources,
		InitializedReportDataSources:              initializedDataSources,
		UninitializedReports:                      uninitializedReports,
		InitializedReports:                        initializedReports,
		UninitializedScheduledReports:             uninitializedScheduledReports,
		InitializedScheduledReports:               initializedScheduledReports,
	}, nil
}

type GetDependentGenerationQueriesStatus struct {
	ViewReportGenerationQueries    []*metering.ReportGenerationQuery
	DynamicReportGenerationQueries []*metering.ReportGenerationQuery
}

func GetDependentGenerationQueries(queryGetter reportGenerationQueryGetter, generationQuery *metering.ReportGenerationQuery) (*GetDependentGenerationQueriesStatus, error) {
	viewQueries, err := GetDependentViewGenerationQueries(queryGetter, generationQuery)
	if err != nil {
		return nil, err
	}
	dynamicQueries, err := GetDependentDynamicGenerationQueries(queryGetter, generationQuery)
	if err != nil {
		return nil, err
	}
	return &GetDependentGenerationQueriesStatus{
		ViewReportGenerationQueries:    viewQueries,
		DynamicReportGenerationQueries: dynamicQueries,
	}, nil
}

func GetDependentViewGenerationQueries(queryGetter reportGenerationQueryGetter, generationQuery *metering.ReportGenerationQuery) ([]*metering.ReportGenerationQuery, error) {
	viewReportQueriesAccumulator := make(map[string]*metering.ReportGenerationQuery)
	err := GetDependentGenerationQueriesMemoized(queryGetter, generationQuery, 0, maxDepth, viewReportQueriesAccumulator, false)
	if err != nil {
		return nil, err
	}

	viewQueries := make([]*metering.ReportGenerationQuery, 0, len(viewReportQueriesAccumulator))
	for _, query := range viewReportQueriesAccumulator {
		viewQueries = append(viewQueries, query)
	}
	return viewQueries, nil
}

func GetDependentDynamicGenerationQueries(queryGetter reportGenerationQueryGetter, generationQuery *metering.ReportGenerationQuery) ([]*metering.ReportGenerationQuery, error) {
	dynamicReportQueriesAccumulator := make(map[string]*metering.ReportGenerationQuery)
	err := GetDependentGenerationQueriesMemoized(queryGetter, generationQuery, 0, maxDepth, dynamicReportQueriesAccumulator, true)
	if err != nil {
		return nil, err
	}

	dynamicQueries := make([]*metering.ReportGenerationQuery, 0, len(dynamicReportQueriesAccumulator))
	for _, query := range dynamicReportQueriesAccumulator {
		dynamicQueries = append(dynamicQueries, query)
	}
	return dynamicQueries, nil
}

type reportGenerationQueryGetter interface {
	getReportGenerationQuery(namespace, name string) (*metering.ReportGenerationQuery, error)
}

type reportGenerationQueryGetterFunc func(string, string) (*metering.ReportGenerationQuery, error)

func (f reportGenerationQueryGetterFunc) getReportGenerationQuery(namespace, name string) (*metering.ReportGenerationQuery, error) {
	return f(namespace, name)
}

func NewReportGenerationQueryListerGetter(lister meteringListers.ReportGenerationQueryLister) reportGenerationQueryGetter {
	return reportGenerationQueryGetterFunc(func(namespace, name string) (*metering.ReportGenerationQuery, error) {
		return lister.ReportGenerationQueries(namespace).Get(name)
	})
}

func NewReportGenerationQueryClientGetter(getter meteringClient.ReportGenerationQueriesGetter) reportGenerationQueryGetter {
	return reportGenerationQueryGetterFunc(func(namespace, name string) (*metering.ReportGenerationQuery, error) {
		return getter.ReportGenerationQueries(namespace).Get(name, metav1.GetOptions{})
	})
}

func GetDependentGenerationQueriesMemoized(queryGetter reportGenerationQueryGetter, generationQuery *metering.ReportGenerationQuery, depth, maxDepth int, queriesAccumulator map[string]*metering.ReportGenerationQuery, dynamicQueries bool) error {
	if depth >= maxDepth {
		return fmt.Errorf("detected a cycle at depth %d for generationQuery %s", depth, generationQuery.Name)
	}
	var queries []string
	if dynamicQueries {
		queries = generationQuery.Spec.DynamicReportQueries
	} else {
		queries = generationQuery.Spec.ReportQueries
	}
	for _, queryName := range queries {
		if _, exists := queriesAccumulator[queryName]; exists {
			continue
		}
		genQuery, err := queryGetter.getReportGenerationQuery(generationQuery.Namespace, queryName)
		if err != nil {
			return err
		}
		err = GetDependentGenerationQueriesMemoized(queryGetter, genQuery, depth+1, maxDepth, queriesAccumulator, dynamicQueries)
		if err != nil {
			return err
		}
		queriesAccumulator[genQuery.Name] = genQuery
	}
	return nil
}

type reportDataSourceGetter interface {
	getReportDataSource(namespace, name string) (*metering.ReportDataSource, error)
}

type reportDataSourceGetterFunc func(string, string) (*metering.ReportDataSource, error)

func (f reportDataSourceGetterFunc) getReportDataSource(namespace, name string) (*metering.ReportDataSource, error) {
	return f(namespace, name)
}

func NewReportDataSourceListerGetter(lister meteringListers.ReportDataSourceLister) reportDataSourceGetter {
	return reportDataSourceGetterFunc(func(namespace, name string) (*metering.ReportDataSource, error) {
		return lister.ReportDataSources(namespace).Get(name)
	})
}

func NewReportDataSourceClientGetter(getter meteringClient.ReportDataSourcesGetter) reportDataSourceGetter {
	return reportDataSourceGetterFunc(func(namespace, name string) (*metering.ReportDataSource, error) {
		return getter.ReportDataSources(namespace).Get(name, metav1.GetOptions{})
	})
}

func GetDependentDataSources(dataSourceGetter reportDataSourceGetter, generationQuery *metering.ReportGenerationQuery) ([]*metering.ReportDataSource, error) {
	dataSources := make([]*metering.ReportDataSource, len(generationQuery.Spec.DataSources))
	for i, dataSourceName := range generationQuery.Spec.DataSources {
		dataSource, err := dataSourceGetter.getReportDataSource(generationQuery.Namespace, dataSourceName)
		if err != nil {
			return nil, err
		}
		dataSources[i] = dataSource
	}
	return dataSources, nil
}

type reportGetter interface {
	getReport(namespace, name string) (*metering.Report, error)
}

type reportGetterFunc func(string, string) (*metering.Report, error)

func (f reportGetterFunc) getReport(namespace, name string) (*metering.Report, error) {
	return f(namespace, name)
}

func NewReportListerGetter(lister meteringListers.ReportLister) reportGetter {
	return reportGetterFunc(func(namespace, name string) (*metering.Report, error) {
		return lister.Reports(namespace).Get(name)
	})
}

func NewReportClientGetter(getter meteringClient.ReportsGetter) reportGetter {
	return reportGetterFunc(func(namespace, name string) (*metering.Report, error) {
		return getter.Reports(namespace).Get(name, metav1.GetOptions{})
	})
}

func GetDependentReports(reportGetter reportGetter, generationQuery *metering.ReportGenerationQuery) ([]*metering.Report, error) {
	reports := make([]*metering.Report, len(generationQuery.Spec.Reports))
	for i, reportName := range generationQuery.Spec.Reports {
		report, err := reportGetter.getReport(generationQuery.Namespace, reportName)
		if err != nil {
			return nil, err
		}
		reports[i] = report
	}
	return reports, nil
}

type scheduledReportGetter interface {
	getScheduledReport(namespace, name string) (*metering.ScheduledReport, error)
}

type scheduledReportGetterFunc func(string, string) (*metering.ScheduledReport, error)

func (f scheduledReportGetterFunc) getScheduledReport(namespace, name string) (*metering.ScheduledReport, error) {
	return f(namespace, name)
}

func NewScheduledReportListerGetter(lister meteringListers.ScheduledReportLister) scheduledReportGetter {
	return scheduledReportGetterFunc(func(namespace, name string) (*metering.ScheduledReport, error) {
		return lister.ScheduledReports(namespace).Get(name)
	})
}

func NewScheduledReportClientGetter(getter meteringClient.ScheduledReportsGetter) scheduledReportGetter {
	return scheduledReportGetterFunc(func(namespace, name string) (*metering.ScheduledReport, error) {
		return getter.ScheduledReports(namespace).Get(name, metav1.GetOptions{})
	})
}

func GetDependentScheduledReports(scheduledReportGetter scheduledReportGetter, generationQuery *metering.ReportGenerationQuery) ([]*metering.ScheduledReport, error) {
	scheduledReports := make([]*metering.ScheduledReport, len(generationQuery.Spec.ScheduledReports))
	for i, scheduledReportName := range generationQuery.Spec.ScheduledReports {
		scheduledReport, err := scheduledReportGetter.getScheduledReport(generationQuery.Namespace, scheduledReportName)
		if err != nil {
			return nil, err
		}
		scheduledReports[i] = scheduledReport
	}
	return scheduledReports, nil
}