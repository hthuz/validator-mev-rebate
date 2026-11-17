import React, { useState, useEffect } from 'react'
import axios from 'axios'
import { MapContainer, TileLayer, Marker, Popup, CircleMarker } from 'react-leaflet'
import 'leaflet/dist/leaflet.css'

const API_BASE = '/api'

function ValidatorMap() {
  const [validators, setValidators] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [epoch, setEpoch] = useState(800)

  useEffect(() => {
    fetchValidators()
  }, [epoch])

  const fetchValidators = async () => {
    setLoading(true)
    setError(null)

    try {
      const response = await axios.get(`${API_BASE}/validators?epoch=${epoch}&with_stake=false`)
      const validatorsWithLocation = response.data.validators.filter(
        v => (v.s_ip_latitude && v.s_ip_longitude) || (v.m_dc_lat && v.m_dc_lon)
      )
      setValidators(validatorsWithLocation)
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
    return <div className="loading">Loading validator map...</div>
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
            min={800}
            max={900}
          />
        </label>
        <button onClick={fetchValidators}>Refresh</button>
        <span style={{ marginLeft: '15px', color: '#666' }}>
          Showing {validators.length} validators with location data
        </span>
      </div>

      <div className="map-container">
        <MapContainer center={[20, 0]} zoom={2} style={{ height: '100%', width: '100%' }}>
          <TileLayer
            attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
            url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
          />
          {validators.map((validator) => {
            const lat = validator.s_ip_latitude || validator.m_dc_lat
            const lon = validator.s_ip_longitude || validator.m_dc_lon
            const stakeSize = Math.log(validator.activatedStake / 1e9 + 1) * 2

            return (
              <CircleMarker
                key={validator.voteAccountPubkey}
                center={[lat, lon]}
                radius={Math.max(stakeSize, 4)}
                fillColor="#667eea"
                color="#fff"
                weight={1}
                opacity={0.8}
                fillOpacity={0.6}
              >
                <Popup>
                  <div style={{ minWidth: '200px' }}>
                    <h4 style={{ marginBottom: '8px' }}>{validator.name || 'Unknown'}</h4>
                    <p style={{ margin: '4px 0', fontSize: '0.85rem' }}>
                      <strong>Stake:</strong> {formatStake(validator.activatedStake)} SOL
                    </p>
                    <p style={{ margin: '4px 0', fontSize: '0.85rem' }}>
                      <strong>Location:</strong> {validator.s_ip_city || validator.m_dc_city},{' '}
                      {validator.s_ip_country || validator.m_dc_country}
                    </p>
                    <p style={{ margin: '4px 0', fontSize: '0.85rem' }}>
                      <strong>Commission:</strong> {validator.commission}%
                    </p>
                    {validator.s_total_apy && (
                      <p style={{ margin: '4px 0', fontSize: '0.85rem' }}>
                        <strong>APY:</strong> {(validator.s_total_apy * 100).toFixed(2)}%
                      </p>
                    )}
                  </div>
                </Popup>
              </CircleMarker>
            )
          })}
        </MapContainer>
      </div>

      <div style={{ background: 'white', padding: '20px', borderRadius: '8px', marginTop: '20px' }}>
        <h3 style={{ marginBottom: '15px' }}>Geographic Distribution</h3>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '15px' }}>
          {Object.entries(
            validators.reduce((acc, v) => {
              const country = v.s_ip_country || v.m_dc_country || 'Unknown'
              acc[country] = (acc[country] || 0) + 1
              return acc
            }, {})
          )
            .sort((a, b) => b[1] - a[1])
            .slice(0, 10)
            .map(([country, count]) => (
              <div key={country} style={{ padding: '10px', background: '#f8f9fa', borderRadius: '4px' }}>
                <div style={{ fontSize: '0.85rem', color: '#666' }}>{country}</div>
                <div style={{ fontSize: '1.5rem', fontWeight: '600' }}>{count}</div>
              </div>
            ))}
        </div>
      </div>
    </div>
  )
}

export default ValidatorMap
