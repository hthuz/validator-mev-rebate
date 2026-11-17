import React, { useState, useEffect } from 'react'
import axios from 'axios'
import { Link } from 'react-router-dom'

const API_BASE = '/api'

function Dashboard() {
  const [stats, setStats] = useState(null)
  const [validators, setValidators] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [epoch, setEpoch] = useState(800)
  const [withStake, setWithStake] = useState(false)
  const [epochRange, setEpochRange] = useState({ start: 800, end: 901 })

  useEffect(() => {
    fetchEpochRange()
  }, [])

  useEffect(() => {
    fetchData()
  }, [epoch, withStake])

  const fetchEpochRange = async () => {
    try {
      const response = await axios.get(`${API_BASE}/epochs`)
      setEpochRange({
        start: response.data.start_epoch,
        end: response.data.end_epoch
      })
      setEpoch(response.data.current_default)
    } catch (err) {
      console.error('Failed to fetch epoch range:', err)
    }
  }

  const fetchData = async () => {
    setLoading(true)
    setError(null)

    try {
      const [statsRes, validatorsRes] = await Promise.all([
        axios.get(`${API_BASE}/stats?epoch=${epoch}`),
        axios.get(`${API_BASE}/validators?epoch=${epoch}&with_stake=${withStake}`)
      ])

      setStats(statsRes.data)
      setValidators(validatorsRes.data.validators)
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  const formatStake = (lamports) => {
    const sol = lamports / 1e9
    return sol.toLocaleString(undefined, { maximumFractionDigits: 0 })
  }

  if (loading && !stats) {
    return <div className="loading">Loading validator data...</div>
  }

  if (error) {
    return <div className="error">Error: {error}</div>
  }

  return (
    <div>
      <div className="controls">
        <label>
          Epoch:
          <input
            type="number"
            value={epoch}
            onChange={(e) => setEpoch(parseInt(e.target.value))}
            min={epochRange.start}
            max={epochRange.end - 1}
          />
        </label>
        <label>
          <input
            type="checkbox"
            checked={withStake}
            onChange={(e) => setWithStake(e.target.checked)}
          />
          Load Stake Details
        </label>
        <button onClick={fetchData} disabled={loading}>
          {loading ? 'Loading...' : 'Refresh'}
        </button>
      </div>

      {stats && (
        <div className="stats-grid">
          <div className="stat-card">
            <h3>Total Validators</h3>
            <div className="value">{stats.total_validators}</div>
          </div>
          <div className="stat-card">
            <h3>Active Validators</h3>
            <div className="value">{stats.active_validators}</div>
          </div>
          <div className="stat-card">
            <h3>Non-Delinquent</h3>
            <div className="value">{stats.nondelinquent_validators}</div>
          </div>
          <div className="stat-card">
            <h3>Total Stake</h3>
            <div className="value">{formatStake(stats.total_stake)} SOL</div>
          </div>
          <div className="stat-card">
            <h3>Stakewiz Tracked</h3>
            <div className="value">{stats.stakewiz_count}</div>
          </div>
          <div className="stat-card">
            <h3>Marinade Tracked</h3>
            <div className="value">{stats.marinade_count}</div>
          </div>
        </div>
      )}

      <div className="validators-table">
        <div className="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Rank</th>
                <th>Name</th>
                <th>Vote Account</th>
                <th>Activated Stake (SOL)</th>
                <th>Commission</th>
                <th>Skip Rate</th>
                <th>Location</th>
                <th>APY</th>
                <th>Status</th>
                {withStake && <th>Stakers</th>}
                {withStake && <th>Gini</th>}
              </tr>
            </thead>
            <tbody>
              {validators.slice(0, 100).map((validator, index) => (
                <tr key={validator.voteAccountPubkey}>
                  <td>{index + 1}</td>
                  <td>
                    <Link
                      to={`/validator/${validator.voteAccountPubkey}?epoch=${epoch}`}
                      className="validator-name"
                    >
                      {validator.name || 'Unknown'}
                    </Link>
                  </td>
                  <td className="truncate" title={validator.voteAccountPubkey}>
                    {validator.voteAccountPubkey.substring(0, 8)}...
                  </td>
                  <td>{formatStake(validator.activatedStake)}</td>
                  <td>{validator.commission}%</td>
                  <td>{(validator.skipRate * 100).toFixed(2)}%</td>
                  <td>
                    {validator.s_ip_city && validator.s_ip_country
                      ? `${validator.s_ip_city}, ${validator.s_ip_country}`
                      : validator.m_dc_city && validator.m_dc_country
                      ? `${validator.m_dc_city}, ${validator.m_dc_country}`
                      : 'N/A'}
                  </td>
                  <td>
                    {validator.s_total_apy
                      ? `${(validator.s_total_apy * 100).toFixed(2)}%`
                      : 'N/A'}
                  </td>
                  <td>
                    {validator.delinquent ? (
                      <span className="badge badge-danger">Delinquent</span>
                    ) : (
                      <span className="badge badge-success">Active</span>
                    )}
                    {validator.s_is_jito && (
                      <span className="badge badge-info" style={{ marginLeft: '4px' }}>Jito</span>
                    )}
                  </td>
                  {withStake && <td>{validator.staker_count || 0}</td>}
                  {withStake && <td>{validator.stake_gini ? validator.stake_gini.toFixed(3) : 'N/A'}</td>}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {validators.length > 100 && (
          <div style={{ padding: '15px', textAlign: 'center', color: '#666' }}>
            Showing top 100 validators. Total: {validators.length}
          </div>
        )}
      </div>
    </div>
  )
}

export default Dashboard
