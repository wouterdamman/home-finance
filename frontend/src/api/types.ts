export type UserRole = 'admin' | 'user'

export interface User {
  id: number
  email: string
  displayName: string
  role: UserRole
  avatarUrl?: string | null
}

export interface Period {
  id: number
  year: number
  month: number
  status: 'open' | 'closed'
  closedAt?: string
  incomeTotalCents?: number
  expenseTotalCents?: number
  surplusCents?: number
}

export interface IncomeEntry {
  id: number
  periodId: number
  sourceId?: number
  label?: string
  amountCents: number
  entryType: 'normal' | 'carryover'
  notes: string
  sortOrder: number
  isItemized: boolean
  transactionsTotalCents: number
  effectiveCents: number
}

export interface Category {
  id: number
  name: string
  defaultAmountCents: number
  isItemized: boolean
  sortOrder: number
}

export interface BudgetLine {
  id: number
  periodId: number
  categoryId?: number
  label?: string
  amountCents: number
  tracksTransactions: boolean
  transactionsTotalCents: number
  effectiveCents: number
  targetCents: number
  sortOrder: number
}

export interface Transaction {
  id: number
  periodId: number
  categoryId: number
  amountCents: number
  description: string
  txDate?: string
}

export interface IncomeTransaction {
  id: number
  periodId: number
  sourceId: number
  amountCents: number
  description: string
  txDate?: string
}

export interface Pot {
  id: number
  name: string
  kind: 'normal' | 'carryover'
  sortOrder: number
}

export interface PotSplit {
  potId: number
  potName: string
  potKind: 'normal' | 'carryover'
  percentage: string
  projectedCents?: number
}

export interface PotBalance {
  potId: number
  name: string
  kind: 'normal' | 'carryover'
  balanceCents: number
  targetCents?: number
  targetDate?: string
}

export interface PotLedgerEntry {
  id: number
  potId: number
  periodId?: number
  entryType: string
  amountCents: number
  description: string
  entryDate: string
  runningBalance: number
}

export interface MonthOverview {
  period: Period
  incomes: IncomeEntry[]
  incomeTotalCents: number
  budgetLines: BudgetLine[]
  expenseTotalCents: number
  surplusCents: number
  splits: PotSplit[]
  splitPercentageTotal: string
}

export interface KidBalance {
  kidId: number
  name: string
  oursCents: number
  theirsCents: number
  totalCents: number
  reportedBalanceCents?: number
  reportedBalanceDate?: string
}

export interface KidLedgerEntry {
  id: number
  kidId: number
  owner: 'ours' | 'theirs'
  entryType: string
  amountCents: number
  description: string
  entryDate: string
  runningOursCents: number
  runningTheirsCents: number
}

export interface YearSummaryMonth {
  month: number
  periodId?: number
  status?: 'open' | 'closed'
  incomeTotalCents: number
  expenseTotalCents: number
  surplusCents: number
}

export interface YearSummary {
  year: number
  locked: boolean
  months: YearSummaryMonth[]
  yearIncomeTotalCents: number
  yearExpenseTotalCents: number
  yearSurplusCents: number
  potBalances: PotBalance[]
}

export interface CategoryTotalsCategory {
  id: number
  name: string
}

export interface CategoryTotalsEntry {
  year: number
  month: number
  values: Record<string, number>
}

export interface CategoryTotals {
  categories: CategoryTotalsCategory[]
  entries: CategoryTotalsEntry[]
}

export interface YearTrend {
  year: number
  incomeTotalCents: number
  expenseTotalCents: number
  surplusCents: number
}

export interface TrendsMonthlyTotal {
  year: number
  month: number
  incomeTotalCents: number
  expenseTotalCents: number
  surplusCents: number
}

export interface IncomeSource {
  id: number
  name: string
  defaultAmountCents: number
  isItemized: boolean
  sortOrder: number
}

export interface ApiError {
  error: { code: string; message: string }
}

export interface ImportReportMonth {
  month: number
  incomeTotalCents: number
  expenseTotalCents: number
  surplusCents: number
  closed: boolean
}

export interface ImportReport {
  months: ImportReportMonth[]
  skippedSheets?: string[]
}

export interface AuditLogEntry {
  id: number
  createdAt: string
  userEmail?: string
  action: string
  entityType?: string
  entityId?: number
  details?: string
}
