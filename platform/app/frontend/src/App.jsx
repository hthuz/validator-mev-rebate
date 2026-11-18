import React, { useState, useEffect } from 'react'
import { BrowserRouter as Router, Routes, Route, Link } from 'react-router-dom'
import Dashboard from './components/Dashboard'
import ValidatorDetail from './components/ValidatorDetail'
import ValidatorMap from './components/ValidatorMap'

function App() {
  return (
    <Router>
      <div className="App">
        <header className="header">
          <div className="container">
            <h1>Solana Validator Risk Assessment System</h1>
            <p>Monitor and analyze Solana validator performance, stake distribution, and geographic data</p>
          </div>
        </header>

        <div className="container">
          <nav style={{ marginBottom: '20px', display: 'flex', gap: '15px' }}>
            <Link to="/" style={{ padding: '8px 16px', background: '#667eea', color: 'white', textDecoration: 'none', borderRadius: '4px' }}>
              Dashboard
            </Link>
            <Link to="/map" style={{ padding: '8px 16px', background: '#667eea', color: 'white', textDecoration: 'none', borderRadius: '4px' }}>
              Validator Map
            </Link>
          </nav>

          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/validator/:voteAccount" element={<ValidatorDetail />} />
            <Route path="/map" element={<ValidatorMap />} />
          </Routes>
        </div>
      </div>
    </Router>
  )
}

export default App
