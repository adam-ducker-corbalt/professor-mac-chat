package chat

// Made-up ProfessorMAC data for trying out the chat. Values are fixed (not
// random) and have a few trends planted on purpose, so you can check whether
// answers are right:
//
//   - MACFin shipped its new pipeline in February 2026: deployments jump from
//     ~2/month to ~15/month, and lead time, failure rate and recovery improve.
//   - MDCT got worse over the last three months (Aug–Oct 2026): change failure
//     rate and recovery time climb.
//   - MACPro improves slowly and steadily all year.
//   - EC2 Graviton migrations are the biggest source of cost savings; one idle
//     RDS database is the single biggest finding.

// Months covered by the DORA data, oldest first.
var sampleMonths = []string{
	"2025-11", "2025-12", "2026-01", "2026-02", "2026-03", "2026-04",
	"2026-05", "2026-06", "2026-07", "2026-08", "2026-09", "2026-10",
}

type Team struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

var sampleTeams = []Team{
	{"MACFin", "Medicaid and CHIP financial reporting. Shipped a new CI/CD pipeline to production in February 2026."},
	{"MACPro", "Medicaid and CHIP program portal for state submissions."},
	{"MDCT", "Medicaid Data Collection Tool for state reporting."},
	{"T-MSIS", "Transformed Medicaid Statistical Information System data pipelines."},
	{"Medicaid.gov", "Public website for Medicaid and CHIP information."},
}

// DoraMonth is one team's DORA metrics for one month.
type DoraMonth struct {
	Team              string  `json:"team"`
	Month             string  `json:"month"`               // YYYY-MM
	Deployments       int     `json:"deployments"`         // production deployments that month
	LeadTimeHours     float64 `json:"lead_time_hours"`     // median commit-to-production time
	ChangeFailureRate float64 `json:"change_failure_rate"` // percent of deployments causing a failure
	RecoveryHours     float64 `json:"recovery_hours"`      // median time to restore service
}

// One value per month in sampleMonths, per metric.
type doraSeries struct {
	deployments []int
	leadTime    []float64
	failureRate []float64
	recovery    []float64
}

var doraByTeam = map[string]doraSeries{
	//           Nov   Dec   Jan   Feb   Mar   Apr   May   Jun   Jul   Aug   Sep   Oct
	"MACFin": {
		deployments: []int{2, 3, 2, 6, 10, 12, 13, 15, 14, 16, 15, 17},
		leadTime:    []float64{240, 216, 252, 120, 60, 48, 42, 38, 40, 36, 34, 32},
		failureRate: []float64{25, 33, 25, 17, 12, 10, 9, 8, 9, 7, 8, 7},
		recovery:    []float64{30, 28, 34, 18, 10, 8, 7, 6, 6, 5, 6, 5},
	},
	"MACPro": {
		deployments: []int{8, 8, 9, 9, 10, 10, 11, 12, 12, 13, 13, 14},
		leadTime:    []float64{72, 70, 68, 66, 62, 60, 58, 56, 54, 52, 50, 48},
		failureRate: []float64{12, 12, 11, 11, 10, 10, 10, 9, 9, 9, 8, 8},
		recovery:    []float64{10, 10, 9, 9, 8, 8, 8, 7, 7, 7, 6, 6},
	},
	"MDCT": {
		deployments: []int{20, 21, 19, 22, 20, 21, 22, 20, 21, 19, 18, 18},
		leadTime:    []float64{24, 24, 26, 23, 24, 25, 24, 23, 25, 30, 34, 36},
		failureRate: []float64{6, 5, 6, 5, 6, 6, 5, 6, 6, 18, 22, 25},
		recovery:    []float64{4, 4, 5, 4, 4, 4, 5, 4, 4, 9, 11, 12},
	},
	"T-MSIS": {
		deployments: []int{4, 5, 4, 4, 5, 4, 5, 4, 5, 4, 5, 4},
		leadTime:    []float64{160, 158, 165, 162, 160, 155, 158, 160, 162, 157, 159, 161},
		failureRate: []float64{15, 16, 15, 14, 15, 16, 15, 15, 14, 15, 16, 15},
		recovery:    []float64{20, 22, 19, 21, 20, 20, 21, 19, 20, 22, 21, 20},
	},
	"Medicaid.gov": {
		deployments: []int{30, 32, 28, 33, 34, 31, 35, 33, 36, 34, 35, 36},
		leadTime:    []float64{12, 12, 13, 11, 12, 12, 11, 12, 11, 12, 11, 11},
		failureRate: []float64{4, 3, 4, 4, 3, 4, 3, 4, 3, 3, 4, 3},
		recovery:    []float64{2, 2, 2, 1.5, 2, 2, 1.5, 2, 2, 1.5, 2, 1.5},
	},
}

// doraMetrics returns every team's monthly metrics, team by team, oldest first.
func doraMetrics() []DoraMonth {
	var rows []DoraMonth
	for _, team := range sampleTeams {
		s := doraByTeam[team.Name]
		for i, month := range sampleMonths {
			rows = append(rows, DoraMonth{
				Team:              team.Name,
				Month:             month,
				Deployments:       s.deployments[i],
				LeadTimeHours:     s.leadTime[i],
				ChangeFailureRate: s.failureRate[i],
				RecoveryHours:     s.recovery[i],
			})
		}
	}
	return rows
}

// Finding is one cost-savings opportunity, like ProfessorMAC's Savings Insights.
type Finding struct {
	ID             int     `json:"id"`
	Team           string  `json:"team"`
	Service        string  `json:"service"` // AWS service: EC2, RDS, ECS Fargate, EBS, S3
	Type           string  `json:"type"`
	Resource       string  `json:"resource"`
	MonthlySavings float64 `json:"monthly_savings_usd"`
	Status         string  `json:"status"` // open, accepted (won't fix), remediated
	Opened         string  `json:"opened"` // YYYY-MM-DD
}

var sampleFindings = []Finding{
	{101, "MACPro", "EC2", "Non-Graviton instance", "macpro-prod-api (m5.4xlarge x6)", 4820, "open", "2026-06-03"},
	{102, "T-MSIS", "EC2", "Non-Graviton instance", "tmsis-etl-workers (r5.8xlarge x4)", 6150, "open", "2026-05-19"},
	{103, "MDCT", "EC2", "Non-Graviton instance", "mdct-batch (c5.2xlarge x8)", 2240, "accepted", "2026-04-11"},
	{104, "MACFin", "EC2", "Underutilized instance", "macfin-reporting-02 (m5.2xlarge)", 610, "open", "2026-08-22"},
	{105, "T-MSIS", "EC2", "Overprovisioned instance", "tmsis-airflow (m5.4xlarge)", 980, "remediated", "2026-03-07"},
	{106, "T-MSIS", "RDS", "No connections (idle)", "tmsis-staging-replica (db.r5.4xlarge)", 6230, "open", "2026-09-02"},
	{107, "MACFin", "RDS", "Non-Graviton instance", "macfin-prod-db (db.m5.2xlarge)", 1340, "open", "2026-07-15"},
	{108, "MACPro", "RDS", "Multi-AZ in non-prod", "macpro-dev-db (db.m5.xlarge)", 520, "remediated", "2026-02-20"},
	{109, "MDCT", "ECS Fargate", "Overprovisioned service", "mdct-forms-api (4 vCPU / 16 GB)", 1870, "open", "2026-08-30"},
	{110, "Medicaid.gov", "ECS Fargate", "Overprovisioned service", "mgov-search (2 vCPU / 8 GB)", 760, "open", "2026-09-18"},
	{111, "MACPro", "EBS", "Not optimized (gp2)", "macpro-prod volumes (14 TB gp2)", 1120, "open", "2026-05-27"},
	{112, "MACFin", "EBS", "Underutilized volume", "macfin-archive-vol (6 TB, 4% used)", 470, "remediated", "2026-01-12"},
	{113, "Medicaid.gov", "S3", "Incomplete multipart uploads", "mgov-assets-bucket", 140, "open", "2026-10-05"},
	{114, "T-MSIS", "Redshift", "Underutilized cluster", "tmsis-analytics (ra3.4xlarge x4)", 3460, "open", "2026-07-08"},
}
