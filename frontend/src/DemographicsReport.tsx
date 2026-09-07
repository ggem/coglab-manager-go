import { useState, type SubmitEvent } from 'react'
import { useQuery } from '@tanstack/react-query'
import { errorMessage, getDemographicsReport } from './api'

export default function DemographicsReport({ experimentId }: { experimentId: number }) {
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')
  const [ran, setRan] = useState(false)

  const {
    data: report,
    isFetching,
    error,
    refetch,
  } = useQuery({
    queryKey: ['demographics-report', experimentId, startDate, endDate],
    queryFn: () => getDemographicsReport(experimentId, startDate, endDate),
    enabled: false,
  })

  function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    setRan(true)
    void refetch()
  }

  return (
    <div>
      <h3>Demographics report</h3>
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
        <>
          <p>
            {report.summary.count} arrived child{report.summary.count === 1 ? '' : 'ren'}
            {report.summary.count > 0 &&
              ` -- age ${report.summary.age_months_min.toFixed(1)}-${report.summary.age_months_max.toFixed(1)} months (avg ${report.summary.age_months_avg.toFixed(1)})`}
          </p>
          <p>
            By sex: {Object.entries(report.summary.by_sex).map(([k, v]) => `${k}: ${v}`).join(', ') || '—'}
          </p>
          <p>
            By race/ethnicity:{' '}
            {Object.entries(report.summary.by_race_ethnicity)
              .map(([k, v]) => `${k}: ${v}`)
              .join(', ') || '—'}
          </p>
          <p>
            By guardian education:{' '}
            {Object.entries(report.summary.by_guardian_education)
              .map(([k, v]) => `${k}: ${v}`)
              .join(', ') || '—'}
          </p>
          <table>
            <caption>Arrived children in range</caption>
            <thead>
              <tr>
                <th>Child</th>
                <th>Sex</th>
                <th>Race/ethnicity</th>
                <th>Schedule date</th>
                <th>Age (months)</th>
                <th>Guardian education</th>
              </tr>
            </thead>
            <tbody>
              {report.children.map((c) => (
                <tr key={c.child_id}>
                  <td>
                    {c.first_name} {c.last_name}
                  </td>
                  <td>{c.sex}</td>
                  <td>{c.race_ethnicity.join(', ')}</td>
                  <td>{c.schedule_date}</td>
                  <td>{c.age_months.toFixed(1)}</td>
                  <td>{c.guardian_education}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
    </div>
  )
}
