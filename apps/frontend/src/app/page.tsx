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

  // Theme & Layout state
  const [theme, setTheme] = useState<'dark' | 'light'>('dark');
  const [layoutMode, setLayoutMode] = useState<'classic' | 'pro'>('pro');

  // Trading States
  const [symbol, setSymbol] = useState('BTC-USDT');
  const [side, setSide] = useState('BUY');
  const [type, setType] = useState('LIMIT');
  const [price, setPrice] = useState('50000');
  const [quantity, setQuantity] = useState('0.1');
  const [timeInForce, setTimeInForce] = useState('GTC');
  const [stopPrice, setStopPrice] = useState('0');
  const [icebergSize, setIcebergSize] = useState('0');

  // Terminal Feeds States
  const [orderStatus, setOrderStatus] = useState('');
  const [openOrders, setOpenOrders] = useState<AdvancedOrder[]>([]);
  const [orderHistory, setOrderHistory] = useState<AdvancedOrder[]>([]);
  const [ticker, setTicker] = useState({ lastPrice: 50000.0, high: 50200.0, low: 49800.0, vol: 120.5, spread: 0.1 });

  // MFA-Specific States
  const [mfaChallengeToken, setMfaChallengeToken] = useState<string | null>(null);
  const [mfaChallengeCode, setMfaChallengeCode] = useState('');
  const [mfaEnabled, setMfaEnabled] = useState(false);
  const [mfaSecret, setMfaSecret] = useState('');
  const [qrCodeUrl, setQrCodeUrl] = useState('');
  const [backupCodes, setBackupCodes] = useState<string[]>([]);
  const [verificationCode, setVerificationCode] = useState('');
  const [disableCode, setDisableCode] = useState('');
  const [showSetup, setShowShowSetup] = useState(false);

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
        if (res.data.mfa_required) {
          // MFA is enabled for this user. Enter intermediate challenge flow.
          setMfaChallengeToken(res.data.mfa_token);
          setError('');
        } else {
          // Standard login direct success
          const { access_token } = res.data;
          setAuth({ id: 'usr_logged_in', email }, access_token);
        }
      }
    } catch (err: any) {
      setError(err.response?.data?.error || 'Authentication action failed');
    } finally {
      setLoading(false);
    }
  };

  const handleMFAChallengeVerify = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');
    try {
      const res = await api.post('/auth/mfa-verify', {
        mfa_token: mfaChallengeToken,
        totp_code: mfaChallengeCode,
      });
      const { access_token } = res.data;
      setAuth({ id: 'usr_logged_in', email }, access_token);
      setMfaChallengeToken(null);
      setMfaChallengeCode('');
    } catch (err: any) {
      setError(err.response?.data?.error || 'Multi-Factor verification failed');
    } finally {
      setLoading(false);
    }
  };

  const fetchMFAStatus = useCallback(async () => {
    if (!accessToken) return;
    try {
      const res = await api.get('/mfa/status');
      setMfaEnabled(res.data.is_mfa_enabled);
    } catch (err) {
      console.error('Failed to fetch MFA status', err);
    }
  }, [accessToken]);

  const initiateMFAEnable = async () => {
    setError('');
    try {
      const res = await api.post('/mfa/enable');
      setMfaSecret(res.data.mfa_secret);
      setQrCodeUrl(res.data.qr_code_url);
      setBackupCodes(res.data.backup_codes || []);
      setShowShowSetup(true);
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to initiate MFA configuration');
    }
  };

  const finalizeMFAEnable = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await api.post('/mfa/verify', { totp_code: verificationCode });
      setMfaEnabled(true);
      setShowShowSetup(false);
      setVerificationCode('');
      setMfaSecret('');
      setQrCodeUrl('');
      setError('MFA Activated successfully! Please store your backup codes safely.');
      fetchMFAStatus();
    } catch (err: any) {
      setError(err.response?.data?.error || 'Invalid verification token. Enrollment failed.');
    } finally {
      setLoading(false);
    }
  };

  const disableMFA = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await api.post('/mfa/disable', { totp_code: disableCode });
      setMfaEnabled(false);
      setDisableCode('');
      setError('MFA disabled successfully.');
      fetchMFAStatus();
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to disable MFA protection');
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

      const tradesRes = await api.get(`/oms/trades?symbol=${symbol}`);
      const t = tradesRes.data.ticker;
      if (t) {
        setTicker({
          lastPrice: t.last_price || 50000.0,
          high: t.high_24h || 50200.0,
          low: t.low_24h || 49800.0,
          vol: t.volume_24h || 120.5,
          spread: t.last_price * 0.0002 || 5.0,
        });
      }
    } catch (err) {
      console.error('Failed to load portfolio feeds', err);
    }
  }, [accessToken, symbol]);

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
      fetchMFAStatus();
      fetchOrders();
      const interval = setInterval(fetchOrders, 3000);
      return () => clearInterval(interval);
    }
  }, [accessToken, fetchMFAStatus, fetchOrders]);

  // Derived financial estimations
  const totalCost = parseFloat(price) * parseFloat(quantity);
  const estimatedFees = totalCost * (side === 'BUY' ? 0.002 : 0.001); // Maker/Taker estimate

  return (
    <main className={`min-h-screen p-6 font-sans transition-colors ${theme === 'dark' ? 'bg-slate-950 text-slate-100' : 'bg-slate-50 text-slate-900'}`}>
      {/* Upper Navigation Header */}
      <header className={`max-w-7xl mx-auto flex justify-between items-center pb-4 border-b ${theme === 'dark' ? 'border-slate-800' : 'border-slate-300'}`}>
        <div className="flex items-center gap-6">
          <h1 className="text-2xl font-extrabold tracking-wider bg-gradient-to-r from-cyan-400 to-blue-600 bg-clip-text text-transparent">
            VELYXORA PRO
          </h1>
          <div className="flex items-center gap-4 bg-slate-900/60 p-1.5 rounded-lg border border-slate-800 text-xs">
            <span className="text-slate-400 font-semibold">{symbol}</span>
            <span className="text-emerald-400 font-bold">${ticker.lastPrice.toFixed(2)}</span>
            <span className="text-slate-500">24h High: ${ticker.high.toFixed(2)}</span>
            <span className="text-slate-500">24h Low: ${ticker.low.toFixed(2)}</span>
            <span className="text-slate-500">Volume: {ticker.vol.toFixed(2)}</span>
          </div>
        </div>
        <div className="flex items-center gap-4">
          {/* Workspace Customizer Controls */}
          <div className="flex items-center gap-1.5 bg-slate-900/60 p-1 rounded-lg border border-slate-800 text-xs">
            <button onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')} className="px-2 py-1 hover:bg-slate-800 rounded">
              {theme === 'dark' ? '☀️ Light' : '🌙 Dark'}
            </button>
            <button onClick={() => setLayoutMode(layoutMode === 'pro' ? 'classic' : 'pro')} className="px-2 py-1 hover:bg-slate-800 rounded">
              Layout: {layoutMode.toUpperCase()}
            </button>
          </div>
          {accessToken ? (
            <div className="flex items-center gap-4">
              <span className="text-xs text-cyan-400">{user?.email}</span>
              <button onClick={logout} className="px-3 py-1.5 bg-rose-950/40 border border-rose-800 text-rose-300 hover:bg-rose-900/50 rounded-lg text-xs font-semibold">
                Logout
              </button>
            </div>
          ) : (
            <span className="text-xs text-slate-500">Not Logged In</span>
          )}
        </div>
      </header>

      {/* Grid Multi-Panel Workspace */}
      <div className="max-w-7xl mx-auto grid grid-cols-1 lg:grid-cols-4 gap-6 mt-6">
        {/* Leftmost Column: Order Entry & Account (1 spans) */}
        <section className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-xl flex flex-col justify-between">
          {!accessToken ? (
            <div>
              {mfaChallengeToken ? (
                // MFA Challenge Authentication Box
                <div>
                  <h2 className="text-lg font-bold mb-4 text-cyan-400 flex items-center gap-2">
                    🔒 Security Verification
                  </h2>
                  <form onSubmit={handleMFAChallengeVerify} className="space-y-4">
                    <p className="text-xs text-slate-400">
                      Multi-Factor Authentication is active on your account. Please input your standard 6-digit authenticator token or backup recovery code to finalize registration.
                    </p>
                    <div>
                      <label className="block text-xs font-semibold text-slate-400 mb-2">Authenticator Code</label>
                      <input
                        type="text"
                        value={mfaChallengeCode}
                        onChange={(e) => setMfaChallengeCode(e.target.value)}
                        className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-slate-100 text-center tracking-widest text-sm font-bold focus:outline-none focus:border-cyan-500"
                        placeholder="000000"
                        maxLength={9}
                        required
                        autoFocus
                      />
                    </div>
                    {error && <p className="text-xs text-amber-400 font-semibold">{error}</p>}
                    <div className="flex gap-2">
                      <button type="submit" className="flex-1 py-2.5 bg-cyan-500 text-slate-950 font-bold rounded-lg text-xs hover:bg-cyan-400">
                        Verify & Login
                      </button>
                      <button type="button" onClick={() => setMfaChallengeToken(null)} className="px-3 py-2.5 bg-slate-950 border border-slate-800 rounded-lg text-slate-300 text-xs hover:bg-slate-900">
                        Back
                      </button>
                    </div>
                  </form>
                </div>
              ) : (
                // Standard register/login panel
                <div>
                  <h2 className="text-lg font-bold mb-4 text-cyan-400">Vault Access Control</h2>
                  <form onSubmit={handleAuth} className="space-y-4">
                    <div>
                      <label className="block text-xs font-semibold text-slate-400 mb-2">Corporate Email</label>
                      <input
                        type="email"
                        value={email}
                        onChange={(e) => setEmail(e.target.value)}
                        className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-slate-100 text-xs focus:outline-none focus:border-cyan-500"
                        placeholder="user@velyxora.com"
                        required
                      />
                    </div>
                    <div>
                      <label className="block text-xs font-semibold text-slate-400 mb-2">Security Passphrase</label>
                      <input
                        type="password"
                        value={password}
                        onChange={(e) => setPassword(e.target.value)}
                        className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-slate-100 text-xs focus:outline-none focus:border-cyan-500"
                        placeholder="••••••••"
                        required
                      />
                    </div>
                    {error && <p className="text-xs text-amber-400 font-semibold">{error}</p>}
                    <button type="submit" className="w-full py-2.5 bg-cyan-500 text-slate-950 font-bold rounded-lg text-xs hover:bg-cyan-400">
                      {isRegister ? 'Register' : 'Authenticate'}
                    </button>
                    <div className="text-center mt-3">
                      <button type="button" onClick={() => setIsRegister(!isRegister)} className="text-xs text-cyan-400 hover:underline">
                        {isRegister ? 'Already have an account? Sign In' : 'Create new corporate profile'}
                      </button>
                    </div>
                  </form>
                </div>
              )}
            </div>
          ) : (
            // Authenticated order form
            <form onSubmit={placeOrder} className="space-y-4">
              <h2 className="text-base font-bold text-cyan-400">Order Placement leg</h2>
              <div className="grid grid-cols-2 gap-2">
                <button
                  type="button"
                  onClick={() => setSide('BUY')}
                  className={`py-1.5 rounded text-xs font-bold transition border ${
                    side === 'BUY' ? 'bg-emerald-950/50 text-emerald-400 border-emerald-500' : 'bg-slate-950 text-slate-400 border-slate-800'
                  }`}
                >
                  Buy
                </button>
                <button
                  type="button"
                  onClick={() => setSide('SELL')}
                  className={`py-1.5 rounded text-xs font-bold transition border ${
                    side === 'SELL' ? 'bg-rose-950/50 text-rose-400 border-rose-500' : 'bg-slate-950 text-slate-400 border-slate-800'
                  }`}
                >
                  Sell
                </button>
              </div>

              <div>
                <label className="block text-[10px] text-slate-400 uppercase font-bold mb-1">Execution Mode</label>
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

              <div className="grid grid-cols-2 gap-2">
                <div>
                  <label className="block text-[10px] text-slate-400 uppercase font-bold mb-1">Price (USDT)</label>
                  <input
                    type="number"
                    value={price}
                    onChange={(e) => setPrice(e.target.value)}
                    className="w-full px-2 py-1 bg-slate-950 border border-slate-800 rounded text-xs text-slate-200 focus:border-cyan-500 focus:outline-none"
                  />
                </div>
                <div>
                  <label className="block text-[10px] text-slate-400 uppercase font-bold mb-1">Quantity</label>
                  <input
                    type="number"
                    step="0.0001"
                    value={quantity}
                    onChange={(e) => setQuantity(e.target.value)}
                    className="w-full px-2 py-1 bg-slate-950 border border-slate-800 rounded text-xs text-slate-200 focus:border-cyan-500 focus:outline-none"
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-2">
                <div>
                  <label className="block text-[10px] text-slate-400 uppercase font-bold mb-1">Stop Trigger</label>
                  <input
                    type="number"
                    value={stopPrice}
                    onChange={(e) => setStopPrice(e.target.value)}
                    className="w-full px-2 py-1 bg-slate-950 border border-slate-800 rounded text-xs text-slate-200"
                  />
                </div>
                <div>
                  <label className="block text-[10px] text-slate-400 uppercase font-bold mb-1">Iceberg visible</label>
                  <input
                    type="number"
                    value={icebergSize}
                    onChange={(e) => setIcebergSize(e.target.value)}
                    className="w-full px-2 py-1 bg-slate-950 border border-slate-800 rounded text-xs text-slate-200"
                  />
                </div>
              </div>

              <div className="border-t border-slate-800 pt-3 space-y-1.5 text-xs text-slate-400">
                <div className="flex justify-between">
                  <span>Subtotal:</span>
                  <span className="font-mono text-slate-200">${totalCost.toFixed(2)} USDT</span>
                </div>
                <div className="flex justify-between">
                  <span>Est. Commissions:</span>
                  <span className="font-mono text-slate-200">${estimatedFees.toFixed(4)} USDT</span>
                </div>
              </div>

              <div className="flex gap-2">
                <button type="submit" className={`flex-1 py-2.5 rounded-lg text-xs font-bold transition text-slate-950 ${side === 'BUY' ? 'bg-emerald-400 hover:bg-emerald-300' : 'bg-rose-400 hover:bg-rose-300'}`}>
                  Place {side} Order
                </button>
                <button type="button" onClick={bulkCancel} className="px-2 py-1 bg-slate-950 border border-rose-800 text-rose-400 text-xs rounded hover:bg-rose-950/30">
                  Cancel All
                </button>
              </div>
            </form>
          )}
        </section>

        {/* Center Panels: Order Book & Trades (2 spans) */}
        <section className="lg:col-span-2 grid grid-cols-1 md:grid-cols-2 gap-4">
          {/* Live Order Book Panel */}
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-4 shadow-lg flex flex-col justify-between">
            <h3 className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">Live Level 2 Order Book</h3>
            <div className="space-y-1 font-mono text-xs">
              <div className="grid grid-cols-3 text-slate-500 font-semibold mb-2 border-b border-slate-800 pb-1">
                <span>Price (USDT)</span>
                <span className="text-right">Size ({symbol.split('-')[0]})</span>
                <span className="text-right">Total (USDT)</span>
              </div>
              {/* Asks (Sell) */}
              <div className="grid grid-cols-3 text-rose-400">
                <span>50100.00</span>
                <span className="text-right">0.25</span>
                <span className="text-right">$12,525</span>
              </div>
              <div className="grid grid-cols-3 text-rose-400 bg-rose-950/20">
                <span>50050.00</span>
                <span className="text-right">1.50</span>
                <span className="text-right">$75,075</span>
              </div>
              {/* Spread Display */}
              <div className="flex justify-between py-2 border-y border-slate-800 my-2 text-xs font-bold">
                <span className="text-slate-400">Spread</span>
                <span className="text-slate-200">${ticker.spread.toFixed(2)} USDT (0.02%)</span>
              </div>
              {/* Bids (Buy) */}
              <div className="grid grid-cols-3 text-emerald-400 bg-emerald-950/20">
                <span>50000.00</span>
                <span className="text-right">0.85</span>
                <span className="text-right">$42,500</span>
              </div>
              <div className="grid grid-cols-3 text-emerald-400">
                <span>49950.00</span>
                <span className="text-right">2.10</span>
                <span className="text-right">$104,895</span>
              </div>
            </div>
          </div>

          {/* Recent Public Trades Feed Panel */}
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-4 shadow-lg">
            <h3 className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">Recent Execution Trades</h3>
            <div className="space-y-2 font-mono text-xs max-h-60 overflow-y-auto">
              <div className="grid grid-cols-3 text-slate-500 font-semibold border-b border-slate-800 pb-1">
                <span>Price</span>
                <span className="text-right">Size</span>
                <span className="text-right">Time</span>
              </div>
              <div className="grid grid-cols-3 text-emerald-400">
                <span>50000.00</span>
                <span className="text-right">0.1250</span>
                <span className="text-right text-slate-500">12:34:56</span>
              </div>
              <div className="grid grid-cols-3 text-rose-400">
                <span>49995.00</span>
                <span className="text-right">0.0500</span>
                <span className="text-right text-slate-500">12:34:49</span>
              </div>
              <div className="grid grid-cols-3 text-emerald-400">
                <span>50002.50</span>
                <span className="text-right">1.2500</span>
                <span className="text-right text-slate-500">12:34:33</span>
              </div>
            </div>
          </div>
        </section>

        {/* Rightmost Column: Portfolio & Alerts (1 spans) */}
        <section className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-xl flex flex-col justify-between">
          <div>
            <h3 className="text-sm font-bold text-slate-300 border-b border-slate-800 pb-2 mb-3">Portfolio Allocations</h3>
            <div className="space-y-4">
              <div className="bg-slate-950 p-3 rounded-lg border border-slate-800/60">
                <span className="block text-[10px] text-slate-500 font-medium">Estimated Balance Sheet (USDT)</span>
                <span className="block text-lg font-bold text-slate-200 mt-1">$15,450.00</span>
              </div>
              <div className="grid grid-cols-2 gap-2 text-xs">
                <div className="bg-slate-950 p-2.5 rounded-lg border border-slate-800/60">
                  <span className="block text-[10px] text-slate-500 font-medium">Available</span>
                  <span className="block font-bold text-emerald-400 mt-1">$9,900.00</span>
                </div>
                <div className="bg-slate-950 p-2.5 rounded-lg border border-slate-800/60">
                  <span className="block text-[10px] text-slate-500 font-medium">Locked</span>
                  <span className="block font-bold text-amber-400 mt-1">$5,550.00</span>
                </div>
              </div>
            </div>
          </div>

          <div className="mt-6">
            <h4 className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">Live Order Status Alerts</h4>
            {orderStatus ? (
              <div className={`p-3 rounded-lg text-xs font-mono border ${orderStatus.startsWith('Error') ? 'bg-rose-950/30 text-rose-400 border-rose-900/60' : 'bg-emerald-950/30 text-emerald-400 border-emerald-900/60'}`}>
                {orderStatus}
              </div>
            ) : (
              <div className="p-3 bg-slate-950 border border-slate-800 rounded-lg text-xs text-slate-500 text-center font-medium">
                No new execution logs
              </div>
            )}
          </div>
        </section>
      </div>

      {/* Advanced Security & MFA Configuration Segment */}
      {accessToken && (
        <section className="max-w-7xl mx-auto bg-slate-900 border border-slate-800 rounded-xl p-5 mt-6 shadow-xl">
          <h3 className="text-base font-bold text-cyan-400 border-b border-slate-800 pb-3 mb-4 flex items-center gap-2">
            🛡️ Corporate Account Security & Multi-Factor Controls
          </h3>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div>
              <p className="text-xs text-slate-400 leading-relaxed mb-4">
                Enforcing strong multi-factor authentication (MFA/TOTP) protects your digital asset custody wallets and trading actions from unauthorized key access.
              </p>
              <div className="bg-slate-950 p-4 rounded-lg border border-slate-800 flex items-center justify-between">
                <div>
                  <span className="text-[10px] text-slate-500 block uppercase font-bold">MFA Protection Level</span>
                  <span className={`text-sm font-bold block mt-1 ${mfaEnabled ? 'text-emerald-400' : 'text-rose-400'}`}>
                    {mfaEnabled ? '✅ HIGHLY SECURED (TOTP ACTIVE)' : '⚠️ UNSECURED (PASSWORD ONLY)'}
                  </span>
                </div>
                {!mfaEnabled ? (
                  <button onClick={initiateMFAEnable} className="px-4 py-2 bg-cyan-500 hover:bg-cyan-400 text-slate-950 font-bold rounded-lg text-xs transition">
                    Configure MFA
                  </button>
                ) : (
                  <div className="text-slate-500 text-xs font-medium">Protected</div>
                )}
              </div>

              {mfaEnabled && (
                <form onSubmit={disableMFA} className="mt-4 p-4 bg-slate-950 rounded-lg border border-slate-800 space-y-3">
                  <h4 className="text-xs font-bold text-rose-400">Deactivate MFA Protection</h4>
                  <p className="text-[11px] text-slate-500">
                    To deactivate MFA, input your standard 6-digit verification code or recovery backup code below:
                  </p>
                  <div className="flex gap-2">
                    <input
                      type="text"
                      value={disableCode}
                      onChange={(e) => setDisableCode(e.target.value)}
                      placeholder="000000"
                      className="px-3 py-1.5 bg-slate-900 border border-slate-800 rounded text-slate-100 text-xs w-28 focus:outline-none"
                    />
                    <button type="submit" className="px-4 py-1.5 bg-rose-900/60 border border-rose-800 text-rose-300 hover:bg-rose-900 text-xs rounded font-bold">
                      Disable MFA
                    </button>
                  </div>
                </form>
              )}
            </div>

            {/* MFA Setup enrollment workflow panel */}
            <div>
              {showSetup ? (
                <form onSubmit={finalizeMFAEnable} className="p-4 bg-slate-950 rounded-lg border border-cyan-800/40 space-y-4">
                  <h4 className="text-xs font-bold text-cyan-400 uppercase tracking-wider">Configure Google Authenticator / Duo</h4>
                  <ol className="list-decimal list-inside text-[11px] text-slate-400 space-y-2">
                    <li>Scan this simulated secure QR block using your mobile authenticator:</li>
                    <div className="my-3 p-3 bg-slate-900 border border-slate-800 rounded-lg flex flex-col items-center gap-2">
                      {/* Interactive visually complete QR rendering block */}
                      <div className="w-32 h-32 bg-slate-100 flex items-center justify-center p-2 rounded-lg border border-slate-300">
                        <div className="w-full h-full bg-slate-950 flex flex-wrap p-2 gap-0.5">
                          {Array.from({ length: 144 }).map((_, i) => (
                            <div key={i} className={`w-2 h-2 ${((i * 17) % 7 === 0 || (i % 9 === 0)) ? 'bg-slate-100' : 'bg-slate-950'}`} />
                          ))}
                        </div>
                      </div>
                      <span className="text-[10px] text-slate-500 break-all max-w-[250px] text-center font-mono">
                        {qrCodeUrl}
                      </span>
                    </div>
                    <li>Or manually input the secret code:</li>
                    <div className="p-2 bg-slate-900 border border-slate-800 rounded font-mono text-xs text-center text-cyan-300 font-bold select-all">
                      {mfaSecret}
                    </div>
                    <li className="text-rose-400 font-bold">Store these 8 Backup Recovery Codes safely. You can use them to recover your account if you lose your phone:</li>
                    <div className="grid grid-cols-2 gap-2 font-mono text-[10px] text-slate-300 bg-slate-900 p-2.5 rounded border border-slate-800">
                      {backupCodes.map((code, idx) => (
                        <div key={idx} className="flex gap-2">
                          <span className="text-slate-500">{idx+1}.</span>
                          <span>{code}</span>
                        </div>
                      ))}
                    </div>
                    <li>Input the 6-digit standard token generated by your device to finalize activation:</li>
                  </ol>

                  <div className="flex gap-2 pt-2">
                    <input
                      type="text"
                      value={verificationCode}
                      onChange={(e) => setVerificationCode(e.target.value)}
                      placeholder="000000"
                      className="px-3 py-2 bg-slate-900 border border-slate-800 rounded text-slate-100 text-xs w-28 text-center font-bold focus:outline-none"
                    />
                    <button type="submit" className="flex-1 py-2 bg-cyan-500 text-slate-950 font-bold rounded text-xs hover:bg-cyan-400 transition">
                      Confirm Activation
                    </button>
                    <button type="button" onClick={() => setShowShowSetup(false)} className="px-3 py-2 bg-slate-900 border border-slate-800 rounded text-slate-400 text-xs hover:bg-slate-850">
                      Cancel
                    </button>
                  </div>
                </form>
              ) : (
                <div className="p-6 bg-slate-950/40 border border-slate-800/60 rounded-xl text-center flex flex-col items-center justify-center min-h-[220px]">
                  <span className="text-2xl mb-2">🔒</span>
                  <span className="text-xs text-slate-400 font-semibold mb-1">MFA Enrollment Terminal</span>
                  <p className="text-[10px] text-slate-500 max-w-xs leading-relaxed">
                    Click &quot;Configure MFA&quot; to trigger secret generation, QR codes, and recover codes.
                  </p>
                </div>
              )}
            </div>
          </div>
        </section>
      )}

      {/* Lower Dashboard: Open & Historical Orders */}
      {accessToken && (
        <section className="max-w-7xl mx-auto bg-slate-900 border border-slate-800 rounded-xl p-5 mt-6 shadow-xl">
          <div className="flex justify-between items-center border-b border-slate-800 pb-3 mb-4">
            <h3 className="text-base font-bold text-cyan-400">Order Management & Audit Ledgers</h3>
            <button onClick={fetchOrders} className="text-xs text-cyan-400 hover:underline font-semibold">Sync Dashboard</button>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            {/* Open Orders Table */}
            <div>
              <h4 className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">Active Open Orders ({openOrders.length})</h4>
              {openOrders.length === 0 ? (
                <p className="text-xs text-slate-500 py-4">No active orders queued</p>
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

            {/* Historical Orders Table */}
            <div>
              <h4 className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">Order Execution History ({orderHistory.length})</h4>
              {orderHistory.length === 0 ? (
                <p className="text-xs text-slate-500 py-4">No historical transactions</p>
              ) : (
                <div className="overflow-x-auto max-h-48">
                  <table className="w-full text-xs text-left">
                    <thead>
                      <tr className="text-slate-500 border-b border-slate-800">
                        <th className="py-2">Side/Type</th>
                        <th>Price</th>
                        <th>Qty/Filled</th>
                        <th>Status</th>
                      </tr>
                    </thead>
                    <tbody>
                      {orderHistory.map((ord) => (
                        <tr key={ord.id} className="border-b border-slate-800/40">
                          <td className="py-2">
                            <span className={ord.side === 'BUY' ? 'text-emerald-400 font-bold' : 'text-rose-400 font-bold'}>{ord.side}</span> {ord.type}
                          </td>
                          <td>{ord.price}</td>
                          <td>{ord.quantity} / {ord.filled_quantity}</td>
                          <td>
                            <span className="px-1.5 py-0.5 rounded bg-slate-950 border border-slate-800 text-[10px] text-slate-400 font-semibold">{ord.status}</span>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          </div>
        </section>
      )}
    </main>
  );
}
