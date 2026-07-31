package layout

// Cassowary constraint solver ported from kasuari 0.4.12 for exact Ratatui layout parity.
// Only the add/fetch path used by layout is implemented (no edit variables).

import "math"

type cStrength float64

const (
	strengthRequired cStrength = 1_001_001_000.0
	strengthStrong   cStrength = 1_000_000.0
	strengthMedium   cStrength = 1_000.0
	strengthWeak     cStrength = 1.0
)

func (s cStrength) value() float64 { return float64(s) }

func clipStrength(v float64) cStrength {
	if v < 0 {
		return 0
	}
	if v > float64(strengthRequired) {
		return strengthRequired
	}
	return cStrength(v)
}

func strengthMul(s cStrength, m float64) cStrength { return clipStrength(s.value() * m) }
func strengthDiv(s cStrength, m float64) cStrength { return clipStrength(s.value() / m) }
func strengthSub(a, b cStrength) cStrength         { return clipStrength(a.value() - b.value()) }

// Layout strength table (ratatui 0.30.2).
var (
	strSpacerSizeEQ   = strengthDiv(strengthRequired, 10)
	strMinSizeGE      = strengthMul(strengthStrong, 100)
	strMaxSizeLE      = strengthMul(strengthStrong, 100)
	strLengthSizeEQ   = strengthMul(strengthStrong, 10)
	strPercentageEQ   = strengthStrong
	strRatioSizeEQ    = strengthDiv(strengthStrong, 10)
	strMinSizeEQ      = strengthMul(strengthMedium, 10)
	strMaxSizeEQ      = strengthMul(strengthMedium, 10)
	strFillGrow       = strengthMedium
	strGrow           = strengthDiv(strengthMedium, 10)
	strSpaceGrow      = strengthMul(strengthWeak, 10)
	strAllSegmentGrow = strengthWeak
)

type cVar int

type cTerm struct {
	v    cVar
	coef float64
}

type cExpr struct {
	terms []cTerm
	c     float64
}

func exprConst(c float64) cExpr { return cExpr{c: c} }
func exprVar(v cVar) cExpr      { return cExpr{terms: []cTerm{{v: v, coef: 1}}} }
func exprSize(start, end cVar) cExpr {
	return cExpr{terms: []cTerm{{v: end, coef: 1}, {v: start, coef: -1}}}
}

func (e cExpr) mul(k float64) cExpr {
	out := cExpr{c: e.c * k, terms: make([]cTerm, len(e.terms))}
	for i, t := range e.terms {
		out.terms[i] = cTerm{v: t.v, coef: t.coef * k}
	}
	return out
}

func (e cExpr) add(o cExpr) cExpr {
	out := cExpr{c: e.c + o.c, terms: make([]cTerm, 0, len(e.terms)+len(o.terms))}
	out.terms = append(out.terms, e.terms...)
	out.terms = append(out.terms, o.terms...)
	return out
}

func (e cExpr) sub(o cExpr) cExpr { return e.add(o.mul(-1)) }
func (e cExpr) subConst(c float64) cExpr {
	e.c -= c
	return e
}

type cRel int

const (
	relLE cRel = iota
	relEQ
	relGE
)

type cConstraint struct {
	expr cExpr
	op   cRel
	str  cStrength
}

func cnLE(lhs cExpr, s cStrength, rhs float64) cConstraint {
	return cConstraint{expr: lhs.subConst(rhs), op: relLE, str: s}
}
func cnGE(lhs cExpr, s cStrength, rhs float64) cConstraint {
	return cConstraint{expr: lhs.subConst(rhs), op: relGE, str: s}
}
func cnEQ(lhs cExpr, s cStrength, rhs float64) cConstraint {
	return cConstraint{expr: lhs.subConst(rhs), op: relEQ, str: s}
}
func cnEQExpr(lhs cExpr, s cStrength, rhs cExpr) cConstraint {
	return cConstraint{expr: lhs.sub(rhs), op: relEQ, str: s}
}

// --- row / symbol ---

type symbolKind uint8

const (
	symInvalid symbolKind = iota
	symExternal
	symSlack
	symError
	symDummy
)

type symbol struct {
	id   int
	kind symbolKind
}

func invalidSymbol() symbol { return symbol{0, symInvalid} }

func nearZero(v float64) bool {
	if v < 0 {
		return -v < 1e-8
	}
	return v < 1e-8
}

type row struct {
	cells map[symbol]float64
	c     float64
}

func newRow(c float64) *row { return &row{cells: make(map[symbol]float64), c: c} }

func (r *row) clone() *row {
	out := newRow(r.c)
	for k, v := range r.cells {
		out.cells[k] = v
	}
	return out
}

func (r *row) add(v float64) float64 {
	r.c += v
	return r.c
}

func (r *row) insertSymbol(s symbol, coef float64) {
	if old, ok := r.cells[s]; ok {
		coef += old
		if nearZero(coef) {
			delete(r.cells, s)
		} else {
			r.cells[s] = coef
		}
		return
	}
	if !nearZero(coef) {
		r.cells[s] = coef
	}
}

func (r *row) insertRow(other *row, coef float64) bool {
	diff := other.c * coef
	r.c += diff
	for s, v := range other.cells {
		r.insertSymbol(s, v*coef)
	}
	return diff != 0
}

func (r *row) remove(s symbol) { delete(r.cells, s) }

func (r *row) reverseSign() {
	r.c = -r.c
	for s, v := range r.cells {
		r.cells[s] = -v
	}
}

func (r *row) solveForSymbol(s symbol) {
	coeff := -1.0 / r.cells[s]
	delete(r.cells, s)
	r.c *= coeff
	for k, v := range r.cells {
		r.cells[k] = v * coeff
	}
}

func (r *row) solveForSymbols(lhs, rhs symbol) {
	r.insertSymbol(lhs, -1)
	r.solveForSymbol(rhs)
}

func (r *row) coefficientFor(s symbol) float64 {
	if v, ok := r.cells[s]; ok {
		return v
	}
	return 0
}

func (r *row) substitute(s symbol, other *row) bool {
	coef, ok := r.cells[s]
	if !ok {
		return false
	}
	delete(r.cells, s)
	return r.insertRow(other, coef)
}

// --- solver ---

type tag struct {
	marker symbol
	other  symbol
}

type varData struct {
	value float64
	sym   symbol
	count int
}

type cSolver struct {
	// constraints keyed by identity index; we don't remove, so simple append tags
	constraintTags []tag
	varData        map[cVar]*varData
	varForSymbol   map[symbol]cVar
	publicChanges  []struct {
		v cVar
		x float64
	}
	changed            map[cVar]struct{}
	shouldClearChanges bool
	rows               map[symbol]*row
	infeasible         []symbol
	objective          *row
	artificial         *row
	idTick             int
	nextVar            cVar
}

func newCSolver() *cSolver {
	return &cSolver{
		varData:      make(map[cVar]*varData),
		varForSymbol: make(map[symbol]cVar),
		changed:      make(map[cVar]struct{}),
		rows:         make(map[symbol]*row),
		objective:    newRow(0),
		idTick:       1,
		nextVar:      1,
	}
}

func (s *cSolver) newVar() cVar {
	v := s.nextVar
	s.nextVar++
	return v
}

func (s *cSolver) varChanged(v cVar) {
	if s.shouldClearChanges {
		clear(s.changed)
		s.shouldClearChanges = false
	}
	s.changed[v] = struct{}{}
}

func (s *cSolver) getVarSymbol(v cVar) symbol {
	if vd, ok := s.varData[v]; ok {
		vd.count++
		return vd.sym
	}
	sym := symbol{s.idTick, symExternal}
	s.idTick++
	s.varForSymbol[sym] = v
	s.varData[v] = &varData{value: math.NaN(), sym: sym, count: 1}
	return sym
}

func (s *cSolver) addConstraint(cn cConstraint) bool {
	r, tg := s.createRow(cn)
	subject := chooseSubject(r, tg)
	if subject.kind == symInvalid && allDummies(r) {
		if !nearZero(r.c) {
			return false
		}
		subject = tg.marker
	}
	if subject.kind == symInvalid {
		if ok := s.addWithArtificial(r); !ok {
			return false
		}
	} else {
		r.solveForSymbol(subject)
		s.substitute(subject, r)
		if subject.kind == symExternal && r.c != 0 {
			s.varChanged(s.varForSymbol[subject])
		}
		s.rows[subject] = r
	}
	s.constraintTags = append(s.constraintTags, tg)
	return s.optimize(false)
}

func (s *cSolver) createRow(cn cConstraint) (*row, tag) {
	r := newRow(cn.expr.c)
	for _, t := range cn.expr.terms {
		if nearZero(t.coef) {
			continue
		}
		sym := s.getVarSymbol(t.v)
		if other, ok := s.rows[sym]; ok {
			r.insertRow(other, t.coef)
		} else {
			r.insertSymbol(sym, t.coef)
		}
	}
	var tg tag
	switch cn.op {
	case relGE, relLE:
		coeff := 1.0
		if cn.op == relGE {
			coeff = -1.0
		}
		slack := symbol{s.idTick, symSlack}
		s.idTick++
		r.insertSymbol(slack, coeff)
		if cn.str < strengthRequired {
			err := symbol{s.idTick, symError}
			s.idTick++
			r.insertSymbol(err, -coeff)
			s.objective.insertSymbol(err, cn.str.value())
			tg = tag{marker: slack, other: err}
		} else {
			tg = tag{marker: slack, other: invalidSymbol()}
		}
	case relEQ:
		if cn.str < strengthRequired {
			ep := symbol{s.idTick, symError}
			s.idTick++
			em := symbol{s.idTick, symError}
			s.idTick++
			r.insertSymbol(ep, -1)
			r.insertSymbol(em, 1)
			s.objective.insertSymbol(ep, cn.str.value())
			s.objective.insertSymbol(em, cn.str.value())
			tg = tag{marker: ep, other: em}
		} else {
			dummy := symbol{s.idTick, symDummy}
			s.idTick++
			r.insertSymbol(dummy, 1)
			tg = tag{marker: dummy, other: invalidSymbol()}
		}
	}
	if r.c < 0 {
		r.reverseSign()
	}
	return r, tg
}

func chooseSubject(r *row, tg tag) symbol {
	// Prefer lowest-id external for determinism (kasuari hash order is unstable across runs).
	best := invalidSymbol()
	for s := range r.cells {
		if s.kind == symExternal && (best.kind == symInvalid || s.id < best.id) {
			best = s
		}
	}
	if best.kind != symInvalid {
		return best
	}
	if (tg.marker.kind == symSlack || tg.marker.kind == symError) && r.coefficientFor(tg.marker) < 0 {
		return tg.marker
	}
	if (tg.other.kind == symSlack || tg.other.kind == symError) && r.coefficientFor(tg.other) < 0 {
		return tg.other
	}
	return invalidSymbol()
}

func allDummies(r *row) bool {
	for s := range r.cells {
		if s.kind != symDummy {
			return false
		}
	}
	return true
}

func (s *cSolver) addWithArtificial(r *row) bool {
	art := symbol{s.idTick, symSlack}
	s.idTick++
	s.rows[art] = r.clone()
	s.artificial = r.clone()
	if !s.optimize(true) {
		s.artificial = nil
		return false
	}
	success := nearZero(s.artificial.c)
	s.artificial = nil
	if rowArt, ok := s.rows[art]; ok {
		delete(s.rows, art)
		if len(rowArt.cells) == 0 {
			return success
		}
		entering := anyPivotable(rowArt)
		if entering.kind == symInvalid {
			return false
		}
		rowArt.solveForSymbols(art, entering)
		s.substitute(entering, rowArt)
		s.rows[entering] = rowArt
	}
	for _, row := range s.rows {
		row.remove(art)
	}
	s.objective.remove(art)
	return success
}

func (s *cSolver) substitute(sym symbol, r *row) {
	for otherSym, otherRow := range s.rows {
		changed := otherRow.substitute(sym, r)
		if otherSym.kind == symExternal && changed {
			s.varChanged(s.varForSymbol[otherSym])
		}
		if otherSym.kind != symExternal && otherRow.c < 0 {
			s.infeasible = append(s.infeasible, otherSym)
		}
	}
	s.objective.substitute(sym, r)
	if s.artificial != nil {
		s.artificial.substitute(sym, r)
	}
}

// optimize: artificial=false uses objective; true uses artificial row
func (s *cSolver) optimize(artificial bool) bool {
	for {
		var objective *row
		if artificial {
			objective = s.artificial
		} else {
			objective = s.objective
		}
		entering := getEnteringSymbol(objective)
		if entering.kind == symInvalid {
			return true
		}
		leaving, row, ok := s.getLeavingRow(entering)
		if !ok {
			return false
		}
		row.solveForSymbols(leaving, entering)
		s.substitute(entering, row)
		if entering.kind == symExternal && row.c != 0 {
			s.varChanged(s.varForSymbol[entering])
		}
		s.rows[entering] = row
	}
}

func getEnteringSymbol(objective *row) symbol {
	// Prefer highest-id negative objective coefficient.
	// Matches kasuari's typical HashMap iteration bias on overconstrained equal-strength
	// ties closely enough for stable ratatui 0.30.2 layouts (e.g. equal Ratio shrink).
	best := invalidSymbol()
	for sym, value := range objective.cells {
		if sym.kind != symDummy && value < 0 {
			if best.kind == symInvalid || sym.id > best.id {
				best = sym
			}
		}
	}
	return best
}

func anyPivotable(r *row) symbol {
	best := invalidSymbol()
	for sym := range r.cells {
		if sym.kind == symSlack || sym.kind == symError {
			if best.kind == symInvalid || sym.id < best.id {
				best = sym
			}
		}
	}
	return best
}

func (s *cSolver) getLeavingRow(entering symbol) (symbol, *row, bool) {
	ratio := math.Inf(1)
	var found symbol
	foundOK := false
	for sym, r := range s.rows {
		if sym.kind == symExternal {
			continue
		}
		temp := r.coefficientFor(entering)
		if temp < 0 {
			tempRatio := -r.c / temp
			if !foundOK || tempRatio < ratio-1e-15 || (math.Abs(tempRatio-ratio) <= 1e-15 && sym.id < found.id) {
				ratio = tempRatio
				found = sym
				foundOK = true
			}
		}
	}
	if !foundOK {
		return invalidSymbol(), nil, false
	}
	row := s.rows[found]
	delete(s.rows, found)
	return found, row, true
}

func (s *cSolver) getValue(v cVar) float64 {
	vd, ok := s.varData[v]
	if !ok {
		return 0
	}
	if r, ok := s.rows[vd.sym]; ok {
		return r.c
	}
	return 0
}

// values returns current values for all known vars (0 if unbound).
func (s *cSolver) values(vars []cVar) []float64 {
	out := make([]float64, len(vars))
	for i, v := range vars {
		out[i] = s.getValue(v)
	}
	return out
}
