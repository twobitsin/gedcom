/*
This is free and unencumbered software released into the public domain. For more
information, see <http://unlicense.org/> or the accompanying UNLICENSE file.
*/

// Package gedcom provides a functions to parse GEDCOM files.
package gedcom

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// A Decoder reads and decodes GEDCOM objects from an input stream.
type Decoder struct {
	r       *bufio.Reader
	parsers []parser
	refs    map[string]interface{}
}

// NewDecoder returns a new decoder that reads r.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		r: bufio.NewReader(r),
	}
}

// Decode reads the next GEDCOM-encoded value from its
// input and stores it in the value pointed to by v.
func (d *Decoder) Decode() (*Gedcom, error) {
	g := &Gedcom{
		Family:     make([]*FamilyRecord, 0),
		Individual: make([]*IndividualRecord, 0),
		Media:      make([]*MediaRecord, 0),
		Repository: make([]*RepositoryRecord, 0),
		Source:     make([]*SourceRecord, 0),
		Submitter:  make([]*SubmitterRecord, 0),
	}

	d.refs = make(map[string]interface{})
	d.parsers = []parser{makeRootParser(d, g)}
	if err := d.scan(g); err != nil {
		return nil, err
	}

	return g, nil
}

func (d *Decoder) scan(g *Gedcom) error {
	s := newScanner(d.r)
	for {
		if !s.next() {
			if s.err != nil {
				return fmt.Errorf("scan error (line:%d, position:%d): %w", s.line, s.offset, s.err)
			}
			break
		}
		d.parsers[len(d.parsers)-1](s.level, s.tag, s.value, s.xref)
	}

	return nil
}

type parser func(level int, tag string, value string, xref string) error

func (d *Decoder) pushParser(p parser) {
	d.parsers = append(d.parsers, p)
}

func (d *Decoder) popParser(level int, tag string, value string, xref string) error {
	n := len(d.parsers) - 1
	if n < 1 {
		panic("unexpected condition: no parser in stack")
	}
	d.parsers = d.parsers[0:n]

	return d.parsers[len(d.parsers)-1](level, tag, value, xref)
}

func (d *Decoder) individual(xref string) *IndividualRecord {
	if xref == "" {
		return &IndividualRecord{}
	}

	ref, found := d.refs[xref].(*IndividualRecord)
	if !found {
		rec := &IndividualRecord{Xref: xref}
		d.refs[rec.Xref] = rec
		return rec
	}
	return ref
}

func (d *Decoder) family(xref string) *FamilyRecord {
	if xref == "" {
		return &FamilyRecord{}
	}

	ref, found := d.refs[xref].(*FamilyRecord)
	if !found {
		rec := &FamilyRecord{Xref: xref}
		d.refs[rec.Xref] = rec
		return rec
	}
	return ref
}

func (d *Decoder) source(xref string) *SourceRecord {
	if xref == "" {
		return &SourceRecord{}
	}

	ref, found := d.refs[xref].(*SourceRecord)
	if !found {
		rec := &SourceRecord{Xref: xref}
		d.refs[rec.Xref] = rec
		return rec
	}
	return ref
}

func (d *Decoder) submitter(xref string) *SubmitterRecord {
	if xref == "" {
		return &SubmitterRecord{}
	}

	ref, found := d.refs[xref].(*SubmitterRecord)
	if !found {
		rec := &SubmitterRecord{Xref: xref}
		d.refs[rec.Xref] = rec
		return rec
	}
	return ref
}

func (d *Decoder) submission(xref string) *SubmissionRecord {
	if xref == "" {
		return &SubmissionRecord{}
	}

	ref, found := d.refs[xref].(*SubmissionRecord)
	if !found {
		rec := &SubmissionRecord{Xref: xref}
		d.refs[rec.Xref] = rec
		return rec
	}
	return ref
}

func (d *Decoder) repository(xref string) *RepositoryRecord {
	if xref == "" {
		return &RepositoryRecord{}
	}

	ref, found := d.refs[xref].(*RepositoryRecord)
	if !found {
		rec := &RepositoryRecord{Xref: xref}
		d.refs[rec.Xref] = rec
		return rec
	}
	return ref
}

func makeRootParser(d *Decoder, g *Gedcom) parser {
	return func(level int, tag string, value string, xref string) error {
		if level == 0 {
			switch tag {
			case "HEAD":
				g.Header = &Header{}
				d.pushParser(makeHeaderParser(d, g.Header, level))
			case "INDI":
				obj := d.individual(xref)
				g.Individual = append(g.Individual, obj)
				d.pushParser(makeIndividualParser(d, obj, level))
			case "SUBM":
				// TODO: parse submitters
				g.Submitter = append(g.Submitter, &SubmitterRecord{})
			case "FAM":
				obj := d.family(xref)
				g.Family = append(g.Family, obj)
				d.pushParser(makeFamilyParser(d, obj, level))
			case "SOUR":
				obj := d.source(xref)
				g.Source = append(g.Source, obj)
				d.pushParser(makeSourceParser(d, obj, level))
			case "REPO":
				obj := d.repository(xref)
				g.Repository = append(g.Repository, obj)
				d.pushParser(makeRepositoryParser(d, obj, level))
			default:
				g.UserDefined = append(g.UserDefined, UserDefinedTag{
					Tag:   tag,
					Value: value,
					Xref:  xref,
					Level: level,
				})
			}
		}
		return nil
	}
}

func makeIndividualParser(d *Decoder, i *IndividualRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "NAME":
			n := &NameRecord{Name: value}
			i.Name = append(i.Name, n)
			d.pushParser(makeNameParser(d, n, level))
		case "SEX":
			i.Sex = value
		case "BIRT", "CHR", "DEAT", "BURI", "CREM", "ADOP", "BAPM", "BARM", "BASM", "BLES", "CHRA", "CONF", "FCOM", "ORDN", "NATU", "EMIG", "IMMI", "CENS", "PROB", "WILL", "GRAD", "RETI", "EVEN":
			e := &EventRecord{Tag: tag, Value: value}
			i.Event = append(i.Event, e)
			d.pushParser(makeEventParser(d, tag, e, level))
		case "CAST", "DSCR", "EDUC", "IDNO", "NATI", "NCHI", "NMR", "OCCU", "PROP", "RELI", "RESI", "SSN", "TITL", "FACT":
			e := &EventRecord{Tag: tag, Value: value}
			i.Attribute = append(i.Attribute, e)
			d.pushParser(makeEventParser(d, tag, e, level))
		case "FAMC":
			family := d.family(stripXref(value))
			f := &FamilyLinkRecord{Family: family}
			i.Parents = append(i.Parents, f)
			d.pushParser(makeFamilyLinkParser(d, f, level))
		case "SUBM":
			submitter := d.submitter(stripXref(value))
			i.Submitter = append(i.Submitter, submitter)
		case "ASSO":
			a := &AssociationRecord{Xref: stripXref(value)}
			i.Association = append(i.Association, a)
			d.pushParser(makeAssociationParser(d, a, level))
		case "ALIA":
			// ALIA support is broken in the wild and should be deprecated as per https://www.tamurajones.net/GEDCOMALIA.xhtml
			// Use ALIA as an alternate name
			if xref == "" && value != "" {
				n := &NameRecord{Name: value}
				i.Name = append(i.Name, n)
			}
		case "RFN":
			i.PermanentRecordFileNumber = value
		case "AFN":
			i.AncestralFileNumber = value
		case "FAMS":
			family := d.family(stripXref(value))
			f := &FamilyLinkRecord{Family: family}
			i.Family = append(i.Family, f)
			d.pushParser(makeFamilyLinkParser(d, f, level))
		case "REFN":
			r := &UserReferenceRecord{Number: value}
			i.UserReference = append(i.UserReference, r)
			d.pushParser(makeUserReferenceParser(d, r, level))
		case "RIN":
			i.AutomatedRecordId = value
		case "CHAN":
			d.pushParser(makeChangeParser(d, &i.Change, level))
		case "NOTE":
			r := &NoteRecord{Note: value}
			i.Note = append(i.Note, r)
			d.pushParser(makeNoteParser(d, r, level))
		case "SOUR":
			c := &CitationRecord{Source: d.source(stripXref(value))}
			i.Citation = append(i.Citation, c)
			d.pushParser(makeCitationParser(d, c, level))
		case "OBJE":
			m := &MediaRecord{Xref: stripXref(value)}
			i.Media = append(i.Media, m)
			d.pushParser(makeMediaParser(d, m, level))
		default:
			i.UserDefined = append(i.UserDefined, UserDefinedTag{
				Tag:   tag,
				Value: value,
				Xref:  xref,
				Level: level,
			})
		}
		return nil
	}
}

func makeNameParser(d *Decoder, n *NameRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {

		case "SOUR":
			c := &CitationRecord{Source: d.source(stripXref(value))}
			n.Citation = append(n.Citation, c)
			d.pushParser(makeCitationParser(d, c, level))
		case "NOTE":
			r := &NoteRecord{Note: value}
			n.Note = append(n.Note, r)
			d.pushParser(makeNoteParser(d, r, level))
		}

		return nil
	}
}

func makeSourceParser(d *Decoder, s *SourceRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "DATA":
			if s.Data == nil {
				s.Data = &SourceDataRecord{}
			}
			d.pushParser(makeSourceDataParser(d, s.Data, level))
		case "TITL":
			s.Title = value
			d.pushParser(makeTextParser(d, &s.Title, level))
		case "ABBR":
			s.FiledBy = value
		case "AUTH":
			s.Originator = value
			d.pushParser(makeTextParser(d, &s.Originator, level))
		case "PUBL":
			s.PublicationFacts = value
			d.pushParser(makeTextParser(d, &s.PublicationFacts, level))
		case "TEXT":
			s.Text = value
			d.pushParser(makeTextParser(d, &s.Text, level))
		case "REPO":
			repo := d.repository(stripXref(value))
			s.Repository = &SourceRepositoryRecord{Repository: repo}
			d.pushParser(makeSourceRepositoruParser(d, s.Repository, level))

		case "REFN":
			r := &UserReferenceRecord{Number: value}
			s.UserReference = append(s.UserReference, r)
			d.pushParser(makeUserReferenceParser(d, r, level))
		case "RIN":
			s.AutomatedRecordId = value
		case "CHAN":
			d.pushParser(makeChangeParser(d, &s.Change, level))
		case "NOTE":
			r := &NoteRecord{Note: value}
			s.Note = append(s.Note, r)
			d.pushParser(makeNoteParser(d, r, level))
		case "OBJE":
			m := &MediaRecord{Xref: stripXref(value)}
			s.Media = append(s.Media, m)
			d.pushParser(makeMediaParser(d, m, level))
		default:
			s.UserDefined = append(s.UserDefined, UserDefinedTag{
				Tag:   tag,
				Value: value,
				Xref:  xref,
				Level: level,
			})
		}

		return nil
	}
}

func makeSourceDataParser(d *Decoder, s *SourceDataRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "EVEN":
			se := &SourceEventRecord{Kind: value}
			s.Event = append(s.Event, se)
			d.pushParser(makeSourceEventParser(d, se, level))
		}

		return nil
	}
}

func makeSourceEventParser(d *Decoder, s *SourceEventRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "DATE":
			s.Date = value
		case "PLAC":
			s.Place = value
		}

		return nil
	}
}

func makeSourceRepositoruParser(d *Decoder, s *SourceRepositoryRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "NOTE":
			r := &NoteRecord{Note: value}
			s.Note = append(s.Note, r)
			d.pushParser(makeNoteParser(d, r, level))
		case "CALN":
			r := &SourceCallNumberRecord{CallNumber: value}
			s.CallNumber = append(s.CallNumber, r)
			d.pushParser(makeSourceCallNumberParser(d, r, level))
		}

		return nil
	}
}

func makeSourceCallNumberParser(d *Decoder, s *SourceCallNumberRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "MEDI":
			s.MediaType = value
		}

		return nil
	}
}

func makeCitationParser(d *Decoder, c *CitationRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "PAGE":
			c.Page = value
		case "QUAY":
			c.Quay = value
		case "NOTE":
			r := &NoteRecord{Note: value}
			c.Note = append(c.Note, r)
			d.pushParser(makeNoteParser(d, r, level))
		case "DATA":
			d.pushParser(makeDataParser(d, &c.Data, level))
		default:
			c.UserDefined = append(c.UserDefined, UserDefinedTag{
				Tag:   tag,
				Value: value,
				Xref:  xref,
				Level: level,
			})

		}

		return nil
	}
}

func makeNoteParser(d *Decoder, n *NoteRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "CONT":
			n.Note = n.Note + "\n" + value
		case "CONC":
			n.Note = n.Note + value
		case "SOUR":
			c := &CitationRecord{Source: d.source(stripXref(value))}
			n.Citation = append(n.Citation, c)
			d.pushParser(makeCitationParser(d, c, level))
		}

		return nil
	}
}

func makeTextParser(d *Decoder, s *string, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "CONT":
			*s = *s + "\n" + value
		case "CONC":
			*s = *s + value
		}

		return nil
	}
}

func makeDataParser(d *Decoder, r *DataRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "DATE":
			r.Date = value
		case "TEXT":
			r.Text = append(r.Text, value)
			d.pushParser(makeTextParser(d, &r.Text[len(r.Text)-1], level))
		}

		return nil
	}
}

func makeEventParser(d *Decoder, parentTag string, e *EventRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}

		// Some special case handling
		switch parentTag {
		case "BIRT", "CHR":
			if tag == "FAMC" {
				family := d.family(stripXref(value))
				e.ChildInFamily = family
				return nil
			}
		case "ADOP":
			if tag == "FAMC" {
				family := d.family(stripXref(value))
				e.ChildInFamily = family
				d.pushParser(makeEventAdoptParser(d, e, level))
				return nil
			}
		}

		switch tag {
		case "TYPE":
			e.Type = value
		case "DATE":
			e.Date = value
		case "PLAC":
			e.Place.Name = value
			d.pushParser(makePlaceParser(d, &e.Place, level))
		case "ADDR":
			e.Address.Full = value
			d.pushParser(makeAddressParser(d, &e.Address, level))
		case "AGNC":
			e.ResponsibleAgency = value
		case "RELI":
			e.ReligiousAffiliation = value
		case "CAUS":
			e.Cause = value
		case "RESN":
			e.RestrictionNotice = value
		case "NOTE":
			r := &NoteRecord{Note: value}
			e.Note = append(e.Note, r)
			d.pushParser(makeNoteParser(d, r, level))
		case "SOUR":
			c := &CitationRecord{Source: d.source(stripXref(value))}
			e.Citation = append(e.Citation, c)
			d.pushParser(makeCitationParser(d, c, level))
		case "OBJE":
			m := &MediaRecord{Xref: stripXref(value)}
			e.Media = append(e.Media, m)
			d.pushParser(makeMediaParser(d, m, level))
		default:

			e.UserDefined = append(e.UserDefined, UserDefinedTag{
				Tag:   tag,
				Value: value,
				Xref:  xref,
				Level: level,
			})
		}

		return nil
	}
}

func makeEventAdoptParser(d *Decoder, e *EventRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}

		switch tag {
		case "ADOP":
			e.AdoptedByParent = value
		}

		return nil
	}
}

func makePlaceParser(d *Decoder, p *PlaceRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {

		case "SOUR":
			c := &CitationRecord{Source: d.source(stripXref(value))}
			p.Citation = append(p.Citation, c)
			d.pushParser(makeCitationParser(d, c, level))
		case "NOTE":
			r := &NoteRecord{Note: value}
			p.Note = append(p.Note, r)
			d.pushParser(makeNoteParser(d, r, level))
		}

		return nil
	}
}

func makeFamilyLinkParser(d *Decoder, f *FamilyLinkRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "PEDI":
			f.Type = value
		case "NOTE":
			r := &NoteRecord{Note: value}
			f.Note = append(f.Note, r)
			d.pushParser(makeNoteParser(d, r, level))

		}

		return nil
	}
}

func makeFamilyParser(d *Decoder, f *FamilyRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "HUSB":
			f.Husband = d.individual(stripXref(value))
		case "WIFE":
			f.Wife = d.individual(stripXref(value))
		case "CHIL":
			f.Child = append(f.Child, d.individual(stripXref(value)))
		case "ANUL", "CENS", "DIV", "DIVF", "ENGA", "MARR", "MARB", "MARC", "MARL", "MARS", "EVEN":
			e := &EventRecord{Tag: tag, Value: value}
			f.Event = append(f.Event, e)
			d.pushParser(makeEventParser(d, tag, e, level))
		case "NCHI":
			f.NumberOfChildren = value
		case "REFN":
			r := &UserReferenceRecord{Number: value}
			f.UserReference = append(f.UserReference, r)
			d.pushParser(makeUserReferenceParser(d, r, level))
		case "RIN":
			f.AutomatedRecordId = value
		case "CHAN":
			d.pushParser(makeChangeParser(d, &f.Change, level))
		case "NOTE":
			r := &NoteRecord{Note: value}
			f.Note = append(f.Note, r)
			d.pushParser(makeNoteParser(d, r, level))
		case "SOUR":
			c := &CitationRecord{Source: d.source(stripXref(value))}
			f.Citation = append(f.Citation, c)
			d.pushParser(makeCitationParser(d, c, level))
		case "OBJE":
			m := &MediaRecord{Xref: stripXref(value)}
			f.Media = append(f.Media, m)
			d.pushParser(makeMediaParser(d, m, level))
		default:
			f.UserDefined = append(f.UserDefined, UserDefinedTag{
				Tag:   tag,
				Value: value,
				Xref:  xref,
				Level: level,
			})
		}
		return nil
	}
}

func makeMediaParser(d *Decoder, m *MediaRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "FILE":
			var f *FileRecord
			if len(m.File) == 0 {
				f = &FileRecord{}
				m.File = append(m.File, f)
			} else {
				f = m.File[len(m.File)-1]
			}
			f.Name = value
			d.pushParser(makeMediaFileParser(d, f, level)) // version 5.5.1
		case "FORM": // version 5.5
			var f *FileRecord
			if len(m.File) == 0 {
				f = &FileRecord{}
				m.File = append(m.File, f)
			} else {
				f = m.File[len(m.File)-1]
			}
			f.Format = value
			d.pushParser(makeMediaFileFormatParser(d, f, level))
		case "TITL": // version 5.5
			var f *FileRecord
			if len(m.File) == 0 {
				f = &FileRecord{}
				m.File = append(m.File, f)
			} else {
				f = m.File[len(m.File)-1]
			}
			f.Title = value
		case "RIN":
			m.AutomatedRecordId = value
		case "REFN":
			r := &UserReferenceRecord{Number: value}
			m.UserReference = append(m.UserReference, r)
			d.pushParser(makeUserReferenceParser(d, r, level))
		case "NOTE":
			r := &NoteRecord{Note: value}
			m.Note = append(m.Note, r)
			d.pushParser(makeNoteParser(d, r, level))
		case "SOUR":
			c := &CitationRecord{Source: d.source(stripXref(value))}
			m.Citation = append(m.Citation, c)
			d.pushParser(makeCitationParser(d, c, level))
		case "CHAN":
			d.pushParser(makeChangeParser(d, &m.Change, level))

		default:
			m.UserDefined = append(m.UserDefined, UserDefinedTag{
				Tag:   tag,
				Value: value,
				Xref:  xref,
				Level: level,
			})
		}

		return nil
	}
}

func makeMediaFileParser(d *Decoder, f *FileRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "FORM":
			f.Format = value
			d.pushParser(makeMediaFileFormatParser(d, f, level))
		case "TITL":
			f.Title = value
		}
		return nil
	}
}

func makeMediaFileFormatParser(d *Decoder, f *FileRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "TYPE":
			f.FormatType = value
		}
		return nil
	}
}

func makeUserReferenceParser(d *Decoder, r *UserReferenceRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "TYPE":
			r.Type = value
		}
		return nil
	}
}

func makeAddressParser(d *Decoder, a *AddressRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "ADDR":
			a.Full = value
			d.pushParser(makeAddressDetailParser(d, a, level))
		case "PHON":
			a.Phone = append(a.Phone, value)
		case "EMAIL":
			a.Email = append(a.Email, value)
		case "FAX":
			a.Fax = append(a.Fax, value)
		case "WWW":
			a.WWW = append(a.WWW, value)

		}

		return nil
	}
}

func makeAddressDetailParser(d *Decoder, a *AddressRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "CONT":
			a.Full = a.Full + "\n" + value
		case "ADR1":
			a.Line1 = value
		case "ADR2":
			a.Line2 = value
		case "CITY":
			a.City = value
		case "STAE":
			a.State = value
		case "POST":
			a.PostalCode = value
		case "CTRY":
			a.Country = value
		}

		return nil
	}
}

func makeHeaderParser(d *Decoder, h *Header, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "SOUR":
			h.SourceSystem.Xref = value
			d.pushParser(makeSystemParser(d, &h.SourceSystem, level))
		case "DEST":
			h.Destination = value
		case "DATE":
			h.Date = value
			d.pushParser(makeHeaderTimeParser(d, h, level))
		case "FILE":
			h.Filename = value
		case "COPR":
			h.Copyright = value
		case "GEDC":
			d.pushParser(makeHeaderVersionParser(d, h, level))
		case "LANG":
			h.Language = value
		case "NOTE":
			h.Note = value
			d.pushParser(makeTextParser(d, &h.Note, level))
		case "SUBM":
			h.Submitter = d.submitter(stripXref(value))
		case "SUBN":
			h.Submission = d.submission(stripXref(value))
		case "CHAR":
			h.CharacterSet = value
			d.pushParser(makeHeaderCharacterSetVersionParser(d, h, level))
		default:
			h.UserDefined = append(h.UserDefined, UserDefinedTag{
				Tag:   tag,
				Value: value,
				Xref:  xref,
				Level: level,
			})
		}
		return nil
	}
}

func makeHeaderTimeParser(d *Decoder, h *Header, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "TIME":
			h.Time = value
		}
		return nil
	}
}

func makeHeaderVersionParser(d *Decoder, h *Header, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "VERS":
			h.Version = value
		case "FORM":
			h.Form = value
		}
		return nil
	}
}

func makeHeaderCharacterSetVersionParser(d *Decoder, h *Header, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "VERS":
			h.CharacterSetVersion = value
		}
		return nil
	}
}

func makeSystemParser(d *Decoder, s *SystemRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "VERS":
			s.Version = value
		case "NAME":
			s.ProductName = value
		case "CORP":
			s.BusinessName = value
			d.pushParser(makeAddressParser(d, &s.Address, level))
		case "DATA":
			s.SourceName = value
			d.pushParser(makeDataSourceParser(d, s, level))
		}
		return nil
	}
}

func makeDataSourceParser(d *Decoder, s *SystemRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "DATE":
			s.SourceDate = value
		case "COPR":
			s.SourceCopyright = value
			d.pushParser(makeTextParser(d, &s.SourceCopyright, level))
		}
		return nil
	}
}

func makeChangeParser(d *Decoder, c *ChangeRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "DATE":
			c.Date = value
			d.pushParser(makeChangeTimeParser(d, c, level))
		case "NOTE":
			r := &NoteRecord{Note: value}
			c.Note = append(c.Note, r)
			d.pushParser(makeNoteParser(d, r, level))
		}

		return nil
	}
}

func makeChangeTimeParser(d *Decoder, c *ChangeRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "Time":
			c.Time = value
		}
		return nil
	}
}

func makeRepositoryParser(d *Decoder, r *RepositoryRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "Name":
			r.Name = value
		case "ADDR":
			r.Address.Full = value
			d.pushParser(makeAddressParser(d, &r.Address, level))
		case "NOTE":
			n := &NoteRecord{Note: value}
			r.Note = append(r.Note, n)
			d.pushParser(makeNoteParser(d, n, level))
		case "RIN":
			r.AutomatedRecordId = value
		case "REFN":
			u := &UserReferenceRecord{Number: value}
			r.UserReference = append(r.UserReference, u)
			d.pushParser(makeUserReferenceParser(d, u, level))
		case "CHAN":
			d.pushParser(makeChangeParser(d, &r.Change, level))
		}
		return nil
	}
}

func makeAssociationParser(d *Decoder, a *AssociationRecord, minLevel int) parser {
	return func(level int, tag string, value string, xref string) error {
		if level <= minLevel {
			return d.popParser(level, tag, value, xref)
		}
		switch tag {
		case "RELA":
			a.Relation = value
		case "SOUR":
			c := &CitationRecord{Source: d.source(stripXref(value))}
			a.Citation = append(a.Citation, c)
			d.pushParser(makeCitationParser(d, c, level))
		case "NOTE":
			r := &NoteRecord{Note: value}
			a.Note = append(a.Note, r)
			d.pushParser(makeNoteParser(d, r, level))
		}

		return nil
	}
}

func stripXref(value string) string {
	return strings.Trim(value, "@")
}
