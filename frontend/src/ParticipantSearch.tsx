import { useState, type SubmitEvent } from 'react'
import { Link } from 'react-router-dom'
import {
  searchChildren,
  searchFamilies,
  errorMessage,
  type ChildSearchResult,
  type FamilySearchResult,
} from './api'

type Tab = 'children' | 'families'

export default function ParticipantSearch() {
  const [tab, setTab] = useState<Tab>('children')
  const [query, setQuery] = useState('')
  const [children, setChildren] = useState<ChildSearchResult[]>([])
  const [families, setFamilies] = useState<FamilySearchResult[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [searched, setSearched] = useState(false)

  async function handleSearch(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      if (tab === 'children') {
        setChildren(await searchChildren(query))
      } else {
        setFamilies(await searchFamilies(query))
      }
      setSearched(true)
    } catch (err) {
      setError(errorMessage(err, 'search failed'))
    } finally {
      setLoading(false)
    }
  }

  function switchTab(next: Tab) {
    setTab(next)
    setError(null)
    setSearched(false)
  }

  return (
    <div className="participant-search">
      <div className="tabs">
        <button
          type="button"
          className={tab === 'children' ? 'active' : ''}
          onClick={() => switchTab('children')}
        >
          Children
        </button>
        <button
          type="button"
          className={tab === 'families' ? 'active' : ''}
          onClick={() => switchTab('families')}
        >
          Families
        </button>
        <Link to="/app/families/new" className="add-family">
          Add family
        </Link>
      </div>
      <form onSubmit={handleSearch}>
        <input
          type="text"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={tab === 'children' ? "Search by child's name…" : "Search by guardian's name…"}
        />
        <button type="submit" disabled={loading}>
          {loading ? 'Searching…' : 'Search'}
        </button>
      </form>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {tab === 'children' ? (
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Sex</th>
              <th>Birth date</th>
              <th>Family ID</th>
            </tr>
          </thead>
          <tbody>
            {children.map((c) => (
              <tr key={c.id}>
                <td>
                  <Link to={`/app/families/${c.family_id}`}>
                    {c.first_name} {c.last_name}
                  </Link>
                </td>
                <td>{c.sex}</td>
                <td>{c.birth_date ?? '—'}</td>
                <td>{c.family_id}</td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : (
        <table>
          <thead>
            <tr>
              <th>Address</th>
              <th>City</th>
              <th>State</th>
              <th>Zip</th>
            </tr>
          </thead>
          <tbody>
            {families.map((f) => (
              <tr key={f.id}>
                <td>
                  <Link to={`/app/families/${f.id}`}>{f.address || `Family #${f.id}`}</Link>
                </td>
                <td>{f.city}</td>
                <td>{f.state}</td>
                <td>{f.zip}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {searched && !loading && tab === 'children' && children.length === 0 && <p>No children found.</p>}
      {searched && !loading && tab === 'families' && families.length === 0 && <p>No families found.</p>}
    </div>
  )
}
