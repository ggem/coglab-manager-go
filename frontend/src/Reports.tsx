import { useState, type SubmitEvent } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useParams } from 'react-router-dom'
import LookupTable from './LookupTable'
import {
  createNewsletter,
  deactivateNewsletter,
  errorMessage,
  exportNewsletterUrl,
  exportNIHReportUrl,
  getHRCReport,
  getZipCodesReport,
  listGrants,
  listNewsletters,
  listRecruitmentSources,
  markNewsletterSent,
  updateNewsletter,
  type Newsletter,
} from './api'

type Tab = 'nih' | 'hrc' | 'zipcodes' | 'newsletters'

const TABS: { key: Tab; label: string }[] = [
  { key: 'nih', label: 'NIH' },
  { key: 'hrc', label: 'HRC' },
  { key: 'zipcodes', label: 'Zip Codes' },
  { key: 'newsletters', label: 'Newsletters' },
]

export default function Reports() {
  const { labId } = useParams<{ labId: string }>()
  const [tab, setTab] = useState<Tab>('nih')
  const id = Number(labId)

  return (
    <div className="reports">
      <div className="tabs">
        {TABS.map((t) => (
          <button key={t.key} type="button" className={tab === t.key ? 'active' : ''} onClick={() => setTab(t.key)}>
            {t.label}
          </button>
        ))}
      </div>

      {tab === 'nih' && <NIHReportTab labId={id} />}
      {tab === 'hrc' && <HRCReportTab labId={id} />}
      {tab === 'zipcodes' && <ZipCodesReportTab labId={id} />}
      {tab === 'newsletters' && <NewslettersTab labId={id} />}
    </div>
  )
}

// Every report tab below shares the same shape: date range (+ maybe one
// more filter) in local state, a "Run report" button that triggers the
// fetch (rather than auto-fetching on every keystroke), and a results
// table -- the same enabled:false + refetch() pattern
// AppointmentScheduleFlow.tsx (FM5) already uses for its own filtered
// search.

// NIH now requires the current-shape participant-level data template
// (encodable only as a CSV, not typed from an on-screen table -- see
// exportNIHReportUrl), so this tab is filters + a download link, the
// same shape NewslettersTab's export control already uses below.
function NIHReportTab({ labId }: { labId: number }) {
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')
  const [grantId, setGrantId] = useState('')

  const { data: grants } = useQuery({ queryKey: ['grants', labId], queryFn: () => listGrants(labId) })
  const canExport = startDate !== '' && endDate !== ''

  return (
    <div className="report-filters">
      <label>
        Start date
        <input type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} required />
      </label>
      <label>
        End date
        <input type="date" value={endDate} onChange={(e) => setEndDate(e.target.value)} required />
      </label>
      <label>
        Grant
        <select value={grantId} onChange={(e) => setGrantId(e.target.value)}>
          <option value="">All grants</option>
          {grants?.map((g) => (
            <option key={g.id} value={g.id}>
              {g.name}
            </option>
          ))}
        </select>
      </label>
      {canExport ? (
        <a href={exportNIHReportUrl(labId, startDate, endDate, grantId === '' ? null : Number(grantId))} className="add-family">
          Download CSV
        </a>
      ) : (
        <button type="button" disabled>
          Download CSV
        </button>
      )}
    </div>
  )
}

function HRCReportTab({ labId }: { labId: number }) {
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')
  const [ran, setRan] = useState(false)

  const {
    data: report,
    isFetching,
    error,
    refetch,
  } = useQuery({
    queryKey: ['hrc-report', labId, startDate, endDate],
    queryFn: () => getHRCReport(labId, startDate, endDate),
    enabled: false,
  })

  function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    setRan(true)
    void refetch()
  }

  return (
    <div>
      <form onSubmit={handleSubmit} className="report-filters">
        <label>
          Start date
          <input type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} required />
        </label>
        <label>
          End date
          <input type="date" value={endDate} onChange={(e) => setEndDate(e.target.value)} required />
        </label>
        <button type="submit" disabled={isFetching}>
          {isFetching ? 'Running…' : 'Run report'}
        </button>
      </form>
      {error && (
        <p className="error" role="alert">
          {errorMessage(error, 'Failed to load report.')}
        </p>
      )}
      {ran && report && (
        <table>
          <caption>HRC protocol report</caption>
          <thead>
            <tr>
              <th>Protocol</th>
              <th>Children</th>
            </tr>
          </thead>
          <tbody>
            {report.protocols.map((p) => (
              <tr key={p.protocol_id ?? 'none'}>
                <td>{p.protocol_name}</td>
                <td>{p.child_count}</td>
              </tr>
            ))}
            <tr>
              <td>
                <strong>Total</strong>
              </td>
              <td>{report.total}</td>
            </tr>
          </tbody>
        </table>
      )}
    </div>
  )
}

function ZipCodesReportTab({ labId }: { labId: number }) {
  const [recruitmentSourceId, setRecruitmentSourceId] = useState('')
  const [ran, setRan] = useState(false)

  const { data: sources } = useQuery({ queryKey: ['recruitment-sources'], queryFn: listRecruitmentSources })
  const {
    data: rows,
    isFetching,
    error,
    refetch,
  } = useQuery({
    queryKey: ['zip-codes-report', labId, recruitmentSourceId],
    queryFn: () => getZipCodesReport(labId, recruitmentSourceId === '' ? null : Number(recruitmentSourceId)),
    enabled: false,
  })

  function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    setRan(true)
    void refetch()
  }

  return (
    <div>
      <form onSubmit={handleSubmit} className="report-filters">
        <label>
          Recruitment source
          <select value={recruitmentSourceId} onChange={(e) => setRecruitmentSourceId(e.target.value)}>
            <option value="">All sources</option>
            {sources?.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>
        </label>
        <button type="submit" disabled={isFetching}>
          {isFetching ? 'Running…' : 'Run report'}
        </button>
      </form>
      {error && (
        <p className="error" role="alert">
          {errorMessage(error, 'Failed to load report.')}
        </p>
      )}
      {ran && rows && (
        <table>
          <caption>Zip codes report</caption>
          <thead>
            <tr>
              <th>Zip</th>
              <th>Priority</th>
              <th>Children</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.zip}>
                <td>{row.zip}</td>
                <td>{row.priority ?? '—'}</td>
                <td>{row.child_count}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

function NewslettersTab({ labId }: { labId: number }) {
  const { data: newsletters } = useQuery({ queryKey: ['newsletters', labId], queryFn: () => listNewsletters(labId) })
  const [newsletterId, setNewsletterId] = useState('')
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')
  const [markResult, setMarkResult] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)

  const markSentMutation = useMutation({
    mutationFn: (vars: { newsletterId: number; startDate: string; endDate: string }) =>
      markNewsletterSent(vars.newsletterId, vars.startDate, vars.endDate),
    // Built from the mutation's own variables, not the live newsletterId/
    // startDate/endDate state -- otherwise the message would silently
    // relabel itself to whatever the user has since typed into the
    // filters, misrepresenting what was actually marked sent.
    onSuccess: (result, vars) => {
      const name = newsletters?.find((n) => n.id === vars.newsletterId)?.name ?? 'newsletter'
      setActionError(null)
      setMarkResult(
        `Marked ${result.marked_sent} famil${result.marked_sent === 1 ? 'y' : 'ies'} as sent for "${name}" (visits ${vars.startDate} to ${vars.endDate}).`,
      )
    },
    onError: (err) => setActionError(errorMessage(err, 'Failed to mark sent.')),
  })

  const canExport = startDate !== '' && endDate !== ''

  return (
    <div>
      <LookupTable<Newsletter>
        queryKey={['newsletters', labId]}
        fields={[{ key: 'name', label: 'Name', type: 'text' }]}
        list={() => listNewsletters(labId)}
        create={(values) => createNewsletter(labId, values.name)}
        update={(rowId, values) => updateNewsletter(rowId, values.name)}
        deactivate={(rowId) => deactivateNewsletter(rowId)}
      />

      <h3>Export / mark sent</h3>
      {actionError && (
        <p className="error" role="alert">
          {actionError}
        </p>
      )}
      {markResult && <p>{markResult}</p>}
      <div className="report-filters">
        <label>
          Newsletter
          <select value={newsletterId} onChange={(e) => setNewsletterId(e.target.value)}>
            <option value="">All families (no newsletter filter)</option>
            {newsletters
              ?.filter((n) => !n.deactivated)
              .map((n) => (
                <option key={n.id} value={n.id}>
                  {n.name}
                </option>
              ))}
          </select>
        </label>
        <label>
          Start date
          <input type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} required />
        </label>
        <label>
          End date
          <input type="date" value={endDate} onChange={(e) => setEndDate(e.target.value)} required />
        </label>
        {canExport ? (
          <a
            href={exportNewsletterUrl(labId, startDate, endDate, newsletterId === '' ? null : Number(newsletterId))}
            className="add-family"
          >
            Download CSV
          </a>
        ) : (
          <button type="button" disabled>
            Download CSV
          </button>
        )}
        <button
          type="button"
          onClick={() => markSentMutation.mutate({ newsletterId: Number(newsletterId), startDate, endDate })}
          disabled={!canExport || newsletterId === '' || markSentMutation.isPending}
        >
          {markSentMutation.isPending ? 'Marking…' : 'Mark Sent'}
        </button>
      </div>
    </div>
  )
}
