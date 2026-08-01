'use client';

import React, { useState, useEffect, useCallback } from 'react';
import { useAuthStore } from '../store/authStore';
import api from '../lib/api';

interface AdvancedOrder {
  id: string;
  client_order_id: string;
  symbol: string;
  side: string;
  type: string;
  price: number;
  quantity: number;
  filled_quantity: number;
  status: string;
  time_in_force: string;
  stop_price?: number;
  iceberg_size?: number;
  created_at: string;
}

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
  const [timeInForce, setTimeInForce] = useState('GTC');

  // Advanced Algorithmic Fields
  const [stopPrice, setStopPrice] = useState('0');
  const [icebergSize, setIcebergSize] = useState('0');

  const [orderStatus, setOrderStatus] = useState('');
  const [openOrders, setOpenOrders] = useState<AdvancedOrder[]>([]);
  const [orderHistory, setOrderHistory] = useState<AdvancedOrder[]>([]);

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

  const fetchOrders = useCallback(async () => {
    if (!accessToken) return;
    try {
      const openRes = await api.get('/oms/open');
      setOpenOrders(openRes.data.orders || []);

      const histRes = await api.get('/oms/history');
      setOrderHistory(histRes.data.orders || []);
    } catch (err) {
      console.error('Failed to load open orders', err);
    }
  }, [accessToken]);

  const placeOrder = async (e: React.FormEvent) => {
    e.preventDefault();
    setOrderStatus('');
    try {
      const res = await api.post('/oms/orders', {
        symbol,
        side,
        type,
        price: parseFloat(price),
        quantity: parseFloat(quantity),
        time_in_force: timeInForce,
        stop_price: parseFloat(stopPrice),
        iceberg_size: parseFloat(icebergSize),
      });
      setOrderStatus(`Success: Order processed. Status: ${res.data.order?.status}`);
      fetchOrders();
    } catch (err: any) {
      setOrderStatus(`Error: ${err.response?.data?.error || 'Failed to process order'}`);
    }
  };

  const cancelOrder = async (id: string) => {
    try {
      await api.delete(`/oms/orders/${id}`);
      setOrderStatus(`Order ${id} cancelled successfully.`);
      fetchOrders();
    } catch (err: any) {
      setOrderStatus(`Error cancelling order: ${err.response?.data?.error || err.message}`);
    }
  };

  const bulkCancel = async () => {
    try {
      await api.post('/oms/orders/cancel-bulk', { symbol });
      setOrderStatus(`Bulk cancel triggered for ${symbol}.`);
      fetchOrders();
    } catch (err: any) {
      setOrderStatus(`Error: ${err.response?.data?.error || err.message}`);
    }
  };

  useEffect(() => {
    if (accessToken) {
      fetchOrders();
    }
  }, [accessToken, fetchOrders]);

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
              <div className="flex justify-between items-center border-b border-slate-800 pb-2">
                <h2 className="text-xl font-bold text-cyan-400">Vault Security Profile</h2>
                <button onClick={fetchOrders} className="text-xs text-cyan-400 underline">Refresh Orders</button>
              </div>
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

              <div className="space-y-4">
                <h3 className="text-sm font-bold text-slate-300">Open Orders ({openOrders.length})</h3>
                {openOrders.length === 0 ? (
                  <p className="text-xs text-slate-500">No active orders queued</p>
                ) : (
                  <div className="overflow-x-auto">
                    <table className="w-full text-xs text-left">
                      <thead>
                        <tr className="text-slate-500 border-b border-slate-800">
                          <th className="py-2">Side/Type</th>
                          <th>Price</th>
                          <th>Qty/Filled</th>
                          <th>Action</th>
                        </tr>
                      </thead>
                      <tbody>
                        {openOrders.map((ord) => (
                          <tr key={ord.id} className="border-b border-slate-800/40">
                            <td className="py-2">
                              <span className={ord.side === 'BUY' ? 'text-emerald-400 font-bold' : 'text-rose-400 font-bold'}>{ord.side}</span> {ord.type}
                            </td>
                            <td>{ord.price}</td>
                            <td>{ord.quantity} / {ord.filled_quantity}</td>
                            <td>
                              <button onClick={() => cancelOrder(ord.id)} className="text-xs text-rose-400 hover:underline">Cancel</button>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </div>
            </div>
          )}
        </section>

        {/* Right Column: Trading Interface */}
        <section className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-xl">
          <h2 className="text-xl font-bold mb-4 text-cyan-400">Advanced OMS Terminal</h2>
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

              <div className="grid grid-cols-3 gap-2">
                <div>
                  <label className="block text-xs font-semibold text-slate-400 tracking-wider mb-1">Type</label>
                  <select
                    value={type}
                    onChange={(e) => setType(e.target.value)}
                    className="w-full px-2 py-1.5 bg-slate-950 border border-slate-800 rounded text-xs text-slate-200"
                  >
                    <option value="LIMIT">LIMIT</option>
                    <option value="MARKET">MARKET</option>
                    <option value="STOP_LIMIT">STOP_LIMIT</option>
                    <option value="STOP_MARKET">STOP_MARKET</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-400 tracking-wider mb-1">TIF</label>
                  <select
                    value={timeInForce}
                    onChange={(e) => setTimeInForce(e.target.value)}
                    className="w-full px-2 py-1.5 bg-slate-950 border border-slate-800 rounded text-xs text-slate-200"
                  >
                    <option value="GTC">GTC</option>
                    <option value="IOC">IOC</option>
                    <option value="FOK">FOK</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-400 tracking-wider mb-1">Stop Price</label>
                  <input
                    type="number"
                    value={stopPrice}
                    onChange={(e) => setStopPrice(e.target.value)}
                    className="w-full px-2 py-1 bg-slate-950 border border-slate-800 rounded text-xs text-slate-200"
                  />
                </div>
              </div>

              <div className="grid grid-cols-3 gap-2">
                <div>
                  <label className="block text-xs font-semibold text-slate-400 tracking-wider mb-1">Price (USDT)</label>
                  <input
                    type="number"
                    value={price}
                    onChange={(e) => setPrice(e.target.value)}
                    className="w-full px-2 py-1 bg-slate-950 border border-slate-800 rounded text-xs text-slate-200"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-400 tracking-wider mb-1">Quantity</label>
                  <input
                    type="number"
                    step="0.0001"
                    value={quantity}
                    onChange={(e) => setQuantity(e.target.value)}
                    className="w-full px-2 py-1 bg-slate-950 border border-slate-800 rounded text-xs text-slate-200"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-400 tracking-wider mb-1">Iceberg Size</label>
                  <input
                    type="number"
                    value={icebergSize}
                    onChange={(e) => setIcebergSize(e.target.value)}
                    className="w-full px-2 py-1 bg-slate-950 border border-slate-800 rounded text-xs text-slate-200"
                  />
                </div>
              </div>

              <div className="flex gap-2">
                <button
                  type="submit"
                  className={`flex-1 py-2.5 rounded-xl font-bold transition text-xs ${
                    side === 'BUY'
                      ? 'bg-emerald-500 hover:bg-emerald-400 text-slate-950 shadow-emerald-950/30'
                      : 'bg-rose-500 hover:bg-rose-400 text-slate-950 shadow-rose-950/30'
                  }`}
                >
                  Place {side} Order
                </button>
                <button
                  type="button"
                  onClick={bulkCancel}
                  className="px-3 py-2 bg-slate-950 border border-rose-800 text-rose-400 rounded-xl hover:bg-rose-950/30 text-xs font-bold transition"
                >
                  Bulk Cancel
                </button>
              </div>

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
