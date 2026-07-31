'use client';

import React, { useState } from 'react';
import { useAuthStore } from '../store/authStore';
import api from '../lib/api';

export default function Home() {
  const { user, accessToken, setAuth, logout } = useAuthStore();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [isRegister, setIsRegister] = useState(false);

  // Trading States
  const [symbol, setSymbol] = useState('BTC-USDT');
  const [side, setSide] = useState('BUY');
  const [type, setType] = useState('LIMIT');
  const [price, setPrice] = useState('50000');
  const [quantity, setQuantity] = useState('0.1');
  const [orderStatus, setOrderStatus] = useState('');

  const handleAuth = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');
    try {
      if (isRegister) {
        await api.post('/auth/register', { email, password });
        setIsRegister(false);
        setError('Registration successful! Please login.');
      } else {
        const res = await api.post('/auth/login', { email, password });
        const { access_token } = res.data;
        setAuth({ id: 'usr_logged_in', email }, access_token);
      }
    } catch (err: any) {
      setError(err.response?.data?.error || 'Authentication action failed');
    } finally {
      setLoading(false);
    }
  };

  const placeOrder = async (e: React.FormEvent) => {
    e.preventDefault();
    setOrderStatus('');
    try {
      const res = await api.post('/trading/orders', {
        symbol,
        side,
        type,
        price: parseFloat(price),
        quantity: parseFloat(quantity),
      });
      setOrderStatus(`Success: Order ID ${res.data.order?.id}`);
    } catch (err: any) {
      setOrderStatus(`Error: ${err.response?.data?.error || 'Failed to place order'}`);
    }
  };

  return (
    <main className="min-h-screen bg-slate-950 text-slate-100 p-8 font-sans">
      <header className="max-w-6xl mx-auto flex justify-between items-center pb-8 border-b border-slate-800">
        <div>
          <h1 className="text-3xl font-extrabold tracking-wider bg-gradient-to-r from-cyan-400 to-blue-600 bg-clip-text text-transparent">
            VELYXORA EXCHANGE
          </h1>
          <p className="text-sm text-slate-400 mt-1">Enterprise Trading System Platform Foundation</p>
        </div>
        <div>
          {accessToken ? (
            <div className="flex items-center gap-4">
              <span className="text-sm text-cyan-400">{user?.email}</span>
              <button
                onClick={logout}
                className="px-4 py-2 bg-rose-950/40 border border-rose-800 text-rose-300 hover:bg-rose-900/50 rounded-lg transition text-sm font-medium"
              >
                Logout
              </button>
            </div>
          ) : (
            <span className="text-sm text-slate-500">Not Logged In</span>
          )}
        </div>
      </header>

      <div className="max-w-6xl mx-auto grid grid-cols-1 md:grid-cols-2 gap-8 mt-12">
        {/* Left Column: Authentication Form / Dashboard Details */}
        <section className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-xl">
          {!accessToken ? (
            <div>
              <h2 className="text-xl font-bold mb-4 text-cyan-400">
                {isRegister ? 'Create Professional Account' : 'Secure Vault Gateway'}
              </h2>
              <form onSubmit={handleAuth} className="space-y-4">
                <div>
                  <label className="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">
                    Corporate Email
                  </label>
                  <input
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    className="w-full px-4 py-3 bg-slate-950 border border-slate-800 rounded-xl text-slate-100 focus:outline-none focus:border-cyan-500 transition text-sm"
                    placeholder="user@velyxora.com"
                    required
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">
                    Security Passphrase
                  </label>
                  <input
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    className="w-full px-4 py-3 bg-slate-950 border border-slate-800 rounded-xl text-slate-100 focus:outline-none focus:border-cyan-500 transition text-sm"
                    placeholder="••••••••"
                    required
                  />
                </div>
                {error && <p className="text-xs text-amber-400 font-semibold">{error}</p>}
                <button
                  type="submit"
                  disabled={loading}
                  className="w-full py-3 bg-gradient-to-r from-cyan-500 to-blue-600 hover:from-cyan-400 hover:to-blue-500 text-slate-950 font-bold rounded-xl transition shadow-lg text-sm"
                >
                  {loading ? 'Processing Cryptography...' : isRegister ? 'Initialize Registration' : 'Authenticate Credentials'}
                </button>
              </form>
              <div className="mt-6 text-center">
                <button
                  onClick={() => setIsRegister(!isRegister)}
                  className="text-xs text-slate-400 hover:text-cyan-400 underline transition"
                >
                  {isRegister ? 'Already have credentials? Authenticate' : 'Request new corporate platform registration'}
                </button>
              </div>
            </div>
          ) : (
            <div className="space-y-6">
              <h2 className="text-xl font-bold text-cyan-400 border-b border-slate-800 pb-2">Vault Security Profile</h2>
              <div className="grid grid-cols-2 gap-4">
                <div className="bg-slate-950 p-4 rounded-xl border border-slate-800/60">
                  <span className="block text-xs text-slate-500 font-medium">Authentication Authority</span>
                  <span className="block text-sm font-bold text-slate-200 mt-1">HS256 Standard JWT</span>
                </div>
                <div className="bg-slate-950 p-4 rounded-xl border border-slate-800/60">
                  <span className="block text-xs text-slate-500 font-medium">Secure Key Ring</span>
                  <span className="block text-sm font-bold text-emerald-400 mt-1">Activated</span>
                </div>
              </div>
              <div className="bg-slate-950 p-4 rounded-xl border border-slate-800/60 space-y-2">
                <h3 className="text-xs font-bold text-slate-400 uppercase tracking-wider">Multi-Factor Authentication</h3>
                <p className="text-xs text-slate-500">MFA setup keys are prepared in core security modules for production activation.</p>
              </div>
            </div>
          )}
        </section>

        {/* Right Column: Trading Interface */}
        <section className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-xl">
          <h2 className="text-xl font-bold mb-4 text-cyan-400">Order Entry (Matching Core Engine)</h2>
          {!accessToken ? (
            <div className="flex flex-col items-center justify-center h-64 text-slate-500 border border-dashed border-slate-800 rounded-xl">
              <p className="text-sm">Please authenticate to gain access to order books</p>
            </div>
          ) : (
            <form onSubmit={placeOrder} className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">Instrument</label>
                  <select
                    value={symbol}
                    onChange={(e) => setSymbol(e.target.value)}
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm focus:outline-none focus:border-cyan-500 text-slate-200"
                  >
                    <option value="BTC-USDT">BTC-USDT</option>
                    <option value="ETH-USDT">ETH-USDT</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">Execution Side</label>
                  <div className="flex gap-2">
                    <button
                      type="button"
                      onClick={() => setSide('BUY')}
                      className={`flex-1 py-2 rounded-lg text-xs font-bold transition border ${
                        side === 'BUY'
                          ? 'bg-emerald-950/50 text-emerald-400 border-emerald-500'
                          : 'bg-slate-950 text-slate-400 border-slate-800'
                      }`}
                    >
                      Buy
                    </button>
                    <button
                      type="button"
                      onClick={() => setSide('SELL')}
                      className={`flex-1 py-2 rounded-lg text-xs font-bold transition border ${
                        side === 'SELL'
                          ? 'bg-rose-950/50 text-rose-400 border-rose-500'
                          : 'bg-slate-950 text-slate-400 border-slate-800'
                      }`}
                    >
                      Sell
                    </button>
                  </div>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">Price (USDT)</label>
                  <input
                    type="number"
                    value={price}
                    onChange={(e) => setPrice(e.target.value)}
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-slate-200 focus:outline-none focus:border-cyan-500"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">Quantity</label>
                  <input
                    type="number"
                    step="0.0001"
                    value={quantity}
                    onChange={(e) => setQuantity(e.target.value)}
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-slate-200 focus:outline-none focus:border-cyan-500"
                  />
                </div>
              </div>

              <button
                type="submit"
                className={`w-full py-3 rounded-xl font-bold transition text-sm ${
                  side === 'BUY'
                    ? 'bg-emerald-500 hover:bg-emerald-400 text-slate-950 shadow-emerald-950/30 shadow-lg'
                    : 'bg-rose-500 hover:bg-rose-400 text-slate-950 shadow-rose-950/30 shadow-lg'
                }`}
              >
                Place {side} Order
              </button>

              {orderStatus && (
                <div
                  className={`p-3 rounded-lg text-xs font-mono border mt-4 ${
                    orderStatus.startsWith('Error')
                      ? 'bg-rose-950/30 text-rose-400 border-rose-900/60'
                      : 'bg-emerald-950/30 text-emerald-400 border-emerald-900/60'
                  }`}
                >
                  {orderStatus}
                </div>
              )}
            </form>
          )}
        </section>
      </div>
    </main>
  );
}
