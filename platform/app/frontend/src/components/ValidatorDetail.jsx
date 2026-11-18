import React, { useState, useEffect } from 'react'
import axios from 'axios'
import { useParams, useSearchParams, Link } from 'react-router-dom'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts'

const API_BASE = '/api'

function ValidatorDetail() {
  const { voteAccount } = useParams()
  const [searchParams] = useSearchParams()
  const epoch = searchParams.get('epoch') || 800

  const [validator, setValidator] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  useEffect(() => {
    fetchValidatorDetail()
  }, [voteAccount, epoch])

  const fetchValidatorDetail = async () => {
    setLoading(true)
    setError(null)

    try {
      const response = await axios.get(`${API_BASE}/validators/${voteAccount}?epoch=${epoch}`)
      setValidator(response.data)
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

  if (loading) {
    return <div className="loading">Loading validator details...</div>
  }

  if (error) {
    return <div className="error">Error: {error}</div>
  }

  if (!validator) {
    return <div className="error">Validator not found</div>
  }

  // Prepare chart data
  const topStakersData = validator.top_stakers.map((staker, index) => ({
    name: `#${index + 1}`,
    staker: staker || 'N/A',
    ratio: (validator.top_n_ratio[index] * 100).toFixed(2)
  }))

  const COLORS = ['#667eea', '#764ba2', '#f093fb', '#4facfe', '#43e97b']

  return (
    <div>
      <Link to={`/?epoch=${epoch}`} style={{ marginBottom: '20px', display: 'inline-block', color: '#667eea' }}>
        ← Back to Dashboard
      </Link>

      <div style={{ background: 'white', padding: '30px', borderRadius: '8px', boxShadow: '0 2px 4px rgba(0,0,0,0.1)', marginBottom: '20px' }}>
        <h2 style={{ marginBottom: '20px', color: '#333' }}>
          {validator.name || 'Unknown Validator'}
        </h2>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))', gap: '20px' }}>
          <div>
            <h3 style={{ fontSize: '1rem', color: '#666', marginBottom: '10px' }}>Basic Information</h3>
            <table style={{ width: '100%', fontSize: '0.9rem' }}>
              <tbody>
                <tr>
                  <td style={{ padding: '8px 0', color: '#666' }}>Vote Account:</td>
                  <td style={{ padding: '8px 0', wordBreak: 'break-all' }}>{validator.voteAccountPubkey}</td>
                </tr>
                <tr>
                  <td style={{ padding: '8px 0', color: '#666' }}>Identity:</td>
                  <td style={{ padding: '8px 0', wordBreak: 'break-all' }}>{validator.identityPubkey}</td>
                </tr>
                <tr>
                  <td style={{ padding: '8px 0', color: '#666' }}>Commission:</td>
                  <td style={{ padding: '8px 0' }}>{validator.commission}%</td>
                </tr>
                <tr>
                  <td style={{ padding: '8px 0', color: '#666' }}>Version:</td>
                  <td style={{ padding: '8px 0' }}>{validator.version}</td>
                </tr>
                <tr>
                  <td style={{ padding: '8px 0', color: '#666' }}>Website:</td>
                  <td style={{ padding: '8px 0' }}>
                    {validator.website ? (
                      <a href={validator.website} target="_blank" rel="noopener noreferrer" style={{ color: '#667eea' }}>
                        {validator.website}
                      </a>
                    ) : 'N/A'}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <div>
            <h3 style={{ fontSize: '1rem', color: '#666', marginBottom: '10px' }}>Stake Information</h3>
            <table style={{ width: '100%', fontSize: '0.9rem' }}>
              <tbody>
                <tr>
                  <td style={{ padding: '8px 0', color: '#666' }}>Activated Stake:</td>
                  <td style={{ padding: '8px 0', fontWeight: '600' }}>{formatStake(validator.activatedStake)} SOL</td>
                </tr>
                <tr>
                  <td style={{ padding: '8px 0', color: '#666' }}>Self Stake:</td>
                  <td style={{ padding: '8px 0' }}>{formatStake(validator.self_stake)} SOL</td>
                </tr>
                <tr>
                  <td style={{ padding: '8px 0', color: '#666' }}>Max Single Stake:</td>
                  <td style={{ padding: '8px 0' }}>{formatStake(validator.max_stake)} SOL</td>
                </tr>
                <tr>
                  <td style={{ padding: '8px 0', color: '#666' }}>Total Stakers:</td>
                  <td style={{ padding: '8px 0' }}>{validator.staker_count}</td>
                </tr>
                <tr>
                  <td style={{ padding: '8px 0', color: '#666' }}>Stake Gini:</td>
                  <td style={{ padding: '8px 0' }}>{validator.stake_gini.toFixed(4)}</td>
                </tr>
                <tr>
                  <td style={{ padding: '8px 0', color: '#666' }}>Skip Rate:</td>
                  <td style={{ padding: '8px 0' }}>{(validator.skipRate * 100).toFixed(2)}%</td>
                </tr>
              </tbody>
            </table>
          </div>

          <div>
            <h3 style={{ fontSize: '1rem', color: '#666', marginBottom: '10px' }}>Location & Network</h3>
            <table style={{ width: '100%', fontSize: '0.9rem' }}>
              <tbody>
                <tr>
                  <td style={{ padding: '8px 0', color: '#666' }}>City:</td>
                  <td style={{ padding: '8px 0' }}>{validator.s_ip_city || validator.m_dc_city || 'N/A'}</td>
                </tr>
                <tr>
                  <td style={{ padding: '8px 0', color: '#666' }}>Country:</td>
                  <td style={{ padding: '8px 0' }}>{validator.s_ip_country || validator.m_dc_country || 'N/A'}</td>
                </tr>
                <tr>
                  <td style={{ padding: '8px 0', color: '#666' }}>ASN:</td>
                  <td style={{ padding: '8px 0' }}>{validator.s_ip_asn || validator.m_dc_asn || 'N/A'}</td>
                </tr>
                <tr>
                  <td style={{ padding: '8px 0', color: '#666' }}>IP Org:</td>
                  <td style={{ padding: '8px 0' }}>{validator.s_ip_org || 'N/A'}</td>
                </tr>
                <tr>
                  <td style={{ padding: '8px 0', color: '#666' }}>Coordinates:</td>
                  <td style={{ padding: '8px 0' }}>
                    {validator.s_ip_latitude && validator.s_ip_longitude
                      ? `${validator.s_ip_latitude.toFixed(4)}, ${validator.s_ip_longitude.toFixed(4)}`
                      : 'N/A'}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <div>
            <h3 style={{ fontSize: '1rem', color: '#666', marginBottom: '10px' }}>Performance</h3>
            <table style={{ width: '100%', fontSize: '0.9rem' }}>
              <tbody>
                <tr>
                  <td style={{ padding: '8px 0', color: '#666' }}>Staking APY:</td>
                  <td style={{ padding: '8px 0' }}>
                    {validator.s_staking_apy ? `${(validator.s_staking_apy * 100).toFixed(2)}%` : 'N/A'}
                  </td>
                </tr>
                <tr>
                  <td style={{ padding: '8px 0', color: '#666' }}>Jito APY:</td>
                  <td style={{ padding: '8px 0' }}>
                    {validator.s_jito_apy ? `${(validator.s_jito_apy * 100).toFixed(2)}%` : 'N/A'}
                  </td>
                </tr>
                <tr>
                  <td style={{ padding: '8px 0', color: '#666' }}>Total APY:</td>
                  <td style={{ padding: '8px 0', fontWeight: '600' }}>
                    {validator.s_total_apy ? `${(validator.s_total_apy * 100).toFixed(2)}%` : 'N/A'}
                  </td>
                </tr>
                <tr>
                  <td style={{ padding: '8px 0', color: '#666' }}>Is Jito:</td>
                  <td style={{ padding: '8px 0' }}>
                    {validator.s_is_jito ? (
                      <span className="badge badge-info">Yes</span>
                    ) : 'No'}
                  </td>
                </tr>
                <tr>
                  <td style={{ padding: '8px 0', color: '#666' }}>Status:</td>
                  <td style={{ padding: '8px 0' }}>
                    {validator.delinquent ? (
                      <span className="badge badge-danger">Delinquent</span>
                    ) : (
                      <span className="badge badge-success">Active</span>
                    )}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <div style={{ background: 'white', padding: '30px', borderRadius: '8px', boxShadow: '0 2px 4px rgba(0,0,0,0.1)', marginBottom: '20px' }}>
        <h3 style={{ marginBottom: '20px', color: '#333' }}>Top 5 Stakers Distribution</h3>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(400px, 1fr))', gap: '30px' }}>
          <div>
            <h4 style={{ fontSize: '0.9rem', color: '#666', marginBottom: '15px' }}>Stake Ratio</h4>
            <ResponsiveContainer width="100%" height={300}>
              <BarChart data={topStakersData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="name" />
                <YAxis />
                <Tooltip />
                <Bar dataKey="ratio" fill="#667eea" />
              </BarChart>
            </ResponsiveContainer>
          </div>

          <div>
            <h4 style={{ fontSize: '0.9rem', color: '#666', marginBottom: '15px' }}>Top Stakers</h4>
            <table style={{ width: '100%', fontSize: '0.9rem' }}>
              <thead>
                <tr style={{ borderBottom: '2px solid #dee2e6' }}>
                  <th style={{ padding: '10px', textAlign: 'left' }}>Rank</th>
                  <th style={{ padding: '10px', textAlign: 'left' }}>Staker</th>
                  <th style={{ padding: '10px', textAlign: 'right' }}>Ratio</th>
                </tr>
              </thead>
              <tbody>
                {topStakersData.map((item, index) => (
                  <tr key={index} style={{ borderBottom: '1px solid #dee2e6' }}>
                    <td style={{ padding: '10px' }}>{item.name}</td>
                    <td style={{ padding: '10px' }}>{item.staker}</td>
                    <td style={{ padding: '10px', textAlign: 'right', fontWeight: '600' }}>{item.ratio}%</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  )
}

export default ValidatorDetail
