import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { URL } from 'node:url'
import ts from 'typescript'

const source = readFileSync(
  new URL('../src/identifiers.ts', import.meta.url),
  'utf8',
)
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext },
}).outputText
const { currencyLabel, shortIssuer, matchesPair } = await import(
  `data:text/javascript,${encodeURIComponent(compiled)}`
)

const pair = {
  base: { currency: 'XRP' },
  quote: {
    currency: '524C555344000000000000000000000000000000',
    issuer: 'rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De',
  },
}

test('readable padded currency code keeps full identity searchable', () => {
  assert.equal(currencyLabel(pair.quote.currency), 'RLUSD')
  assert.equal(shortIssuer(pair.quote.issuer), 'rMxCKbE…J8m5De')
  for (const query of [
    'rlusd',
    'XRP/RLUSD',
    pair.quote.currency,
    pair.quote.issuer,
  ]) {
    assert.equal(matchesPair(pair, query), true, query)
  }
})

test('opaque codes stay distinct and do not decode misleading text', () => {
  assert.equal(
    currencyLabel('0102000000000000000000000000000000000000'),
    '01020000…000000',
  )
  assert.equal(
    currencyLabel('5445535400010000000000000000000000000000'),
    '54455354…000000',
  )
  assert.equal(currencyLabel('usd'), 'usd')
  assert.equal(matchesPair(pair, 'unrelated'), false)
})
