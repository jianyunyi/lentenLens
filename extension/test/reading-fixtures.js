// Human-checked fixture explanations, not live model outputs.
const samples = [
  {
    name: 'simple', file: 'simple.go', functionName: 'DoubleSample',
    source: 'package main\nfunc DoubleSample(n int) int { return n * 2 }\n',
    claims: [
      { section:'purpose', text:'将输入整数放大为两倍。' },
      { section:'contract', text:'输入和输出都是 int；此实现没有处理整数溢出。' },
    ],
  },
  {
    name: 'error', file: 'divide.go', functionName: 'DivideSample',
    source: 'package main\nimport "errors"\nfunc DivideSample(a, b int) (int, error) {\n if b == 0 { return 0, errors.New("zero divisor") }\n return a / b, nil\n}\n',
    claims: [
      { section:'purpose', text:'执行整数除法，并向调用者报告除数为零。' },
      { section:'contract', text:'除数为零时返回 0 和错误；正常路径返回商与 nil。' },
      { section:'flow', text:'先检查除数；为零时立即返回错误。' },
      { section:'flow', text:'正常路径执行整数除法并返回结果。' },
      { section:'design', text:'提前返回让后续除法只处理非零除数。', kind:'alternative' },
    ],
  },
  {
    name: 'dependency', file: 'checkout.go', functionName: 'CheckoutSample',
    source: 'package main\nfunc CheckoutSample(price, count int) int {\n if price <= 0 || count <= 0 { return 0 }\n return discountSample(price * count, count)\n}\nfunc discountSample(subtotal, count int) int {\n if count >= 10 { return subtotal * 90 / 100 }\n return subtotal\n}\n',
    claims: [
      { section:'purpose', text:'计算订单金额，满 10 件按九折结算。', dependency:true },
      { section:'contract', text:'非正数价格或数量返回 0；没有处理整数溢出。' },
      { section:'flow', text:'检查价格和数量，拒绝非正数输入。' },
      { section:'flow', text:'计算小计，再将数量与小计传给折扣函数。', dependency:true },
      { section:'design', text:'提前返回让金额计算只处理正数输入。', kind:'alternative' },
      { section:'design', text:'折扣分支位于辅助函数，订单函数负责输入检查和小计。', kind:'alternative', dependency:true },
      { section:'dependency', text:'discountSample 接收小计与数量，应用折扣；返回值直接作为订单总额。', dependency:true },
    ],
    unknowns:['没有记录业务为何选择 10 件和九折，不能据此推断作者动机。'],
  },
];

function explanation(sample, evidence) {
  return {
    id: sample.name, created:'2026-10-15T00:00:00Z', freshness:'已引用证据未变', evidence,
    document: { claims: sample.claims.map((c,i) => ({ id:'c'+i, section:c.section, text:c.text, kind:c.kind || 'behavior', status:'supported', evidence_ids:(c.dependency ? evidence : evidence.slice(0,1)).map(e => e.id) })), unknowns:sample.unknowns || [] },
  };
}
module.exports = { samples, explanation };
