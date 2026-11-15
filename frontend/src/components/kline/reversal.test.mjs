import test from 'node:test'
import assert from 'node:assert/strict'
import { reversalSignals } from './reversal.js'

function bars(length) {
  return {
    opens: Array(length).fill(100),
    highs: Array(length).fill(102),
    lows: Array(length).fill(98),
    closes: Array(length).fill(100),
    vols: Array(length).fill(100),
  }
}

function run(data, enabled, rsi = []) {
  return reversalSignals(data, rsi, enabled)
}

function mirror(data) {
  return {
    opens: data.opens.map(v => -v),
    highs: data.lows.map(v => -v),
    lows: data.highs.map(v => -v),
    closes: data.closes.map(v => -v),
    vols: [...data.vols],
  }
}

test('假跌破和假突破只在收回时确认，未收回可失效', () => {
  const b = bars(23)
  b.lows[20] = 96
  b.closes[20] = 97
  b.closes[21] = 99
  b.highs[22] = 104
  b.closes[22] = 103
  assert.deepEqual(run(b, { falseBreak: true }), [
    { type: 'falseBreak', direction: 'bull', phase: 'warning', index: 20 },
    { type: 'falseBreak', direction: 'bull', phase: 'confirm', index: 21 },
    { type: 'falseBreak', direction: 'bear', phase: 'warning', index: 22 },
  ])
  b.closes[21] = 95
  assert.ok(run(b, { falseBreak: true }).some(e => e.direction === 'bull' && e.phase === 'invalid' && e.index === 21))
})

test('假跌破当根收回直接确认，等待超时标记失效', () => {
  const b = bars(32)
  b.lows[20] = 96
  assert.ok(run(b, { falseBreak: true }).some(e => e.phase === 'confirm' && e.index === 20))
  b.closes[20] = 97
  b.closes.fill(97, 21)
  assert.ok(run(b, { falseBreak: true }).some(e => e.phase === 'invalid' && e.index === 31))
})

test('RSI失败摆动区分确认和失效，顶部规则对称', () => {
  const rsi = [25, 31, 40, 36, 35, 37, 41]
  assert.deepEqual(run(bars(rsi.length), { rsiSwing: true }, rsi), [
    { type: 'rsiSwing', direction: 'bull', phase: 'warning', index: 5 },
    { type: 'rsiSwing', direction: 'bull', phase: 'confirm', index: 6 },
  ])
  const bear = rsi.map(v => 100 - v)
  assert.deepEqual(run(bars(bear.length), { rsiSwing: true }, bear).map(e => e.direction), ['bear', 'bear'])
  rsi[6] = 24
  assert.equal(run(bars(rsi.length), { rsiSwing: true }, rsi).at(-1).phase, 'invalid')
})

test('衰竭量要求此前均量有效且后续突破预警K线', () => {
  const b = bars(22)
  b.closes[15] = 110
  b.opens[20] = 100
  b.highs[20] = 101
  b.lows[20] = 94
  b.closes[20] = 99
  b.vols[20] = 200
  b.closes[21] = 102
  assert.deepEqual(run(b, { exhaustion: true }).filter(e => e.direction === 'bull').map(e => [e.phase, e.index]), [
    ['warning', 20], ['confirm', 21],
  ])
  assert.deepEqual(run(mirror(b), { exhaustion: true }).filter(e => e.direction === 'bear').map(e => [e.phase, e.index]), [
    ['warning', 20], ['confirm', 21],
  ])
  b.vols.fill(0, 0, 20)
  assert.equal(run(b, { exhaustion: true }).length, 0)
})

test('双底需右侧两根确认，之后才可突破颈线', () => {
  const b = bars(13)
  b.lows[3] = 90
  b.lows[9] = 91
  b.closes[12] = 103
  b.highs[12] = 104
  assert.deepEqual(run(b, { double: true }).filter(e => e.direction === 'bull').map(e => [e.phase, e.index]), [
    ['warning', 11], ['confirm', 12],
  ])
  assert.deepEqual(run(mirror(b), { double: true }).filter(e => e.direction === 'bear').map(e => [e.phase, e.index]), [
    ['warning', 11], ['confirm', 12],
  ])
  b.closes[10] = 103
  assert.equal(run(b, { double: true }).filter(e => e.direction === 'bull').length, 0)
})

test('历史前缀的事件不会被后续K线改写', () => {
  const b = bars(35)
  b.lows[20] = 96
  b.closes[20] = 97
  b.closes[21] = 99
  b.highs[25] = 106
  b.closes[25] = 105
  const enabled = { falseBreak: true, exhaustion: true, double: true, rsiSwing: true }
  const rsi = Array(35).fill(50)
  rsi.splice(23, 7, 25, 31, 40, 36, 35, 37, 41)
  const full = run(b, enabled, rsi)
  for (let n = 1; n <= b.closes.length; n++) {
    const prefix = Object.fromEntries(Object.entries(b).map(([key, value]) => [key, value.slice(0, n)]))
    assert.deepEqual(run(prefix, enabled, rsi.slice(0, n)), full.filter(event => event.index < n))
  }
})
