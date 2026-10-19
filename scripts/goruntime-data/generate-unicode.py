"""Generate owned Go Unicode tables from verified Unicode 17 inputs, offline."""
import argparse
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BINARY = '''ASCII ASCII_Hex_Digit Alphabetic Any Assigned Bidi_Control Bidi_Mirrored Case_Ignorable Cased Changes_When_Casefolded Changes_When_Casemapped Changes_When_Lowercased Changes_When_NFKC_Casefolded Changes_When_Titlecased Changes_When_Uppercased Dash Default_Ignorable_Code_Point Deprecated Diacritic Emoji Emoji_Component Emoji_Modifier Emoji_Modifier_Base Emoji_Presentation Extended_Pictographic Extender Grapheme_Base Grapheme_Extend Hex_Digit IDS_Binary_Operator IDS_Trinary_Operator ID_Continue ID_Start Ideographic Join_Control Logical_Order_Exception Lowercase Math Noncharacter_Code_Point Pattern_Syntax Pattern_White_Space Quotation_Mark Radical Regional_Indicator Sentence_Terminal Soft_Dotted Terminal_Punctuation Unified_Ideograph Uppercase Variation_Selector White_Space XID_Continue XID_Start'''.split()
HEADER = '// Code generated from Unicode 17.0.0 data. DO NOT EDIT.\n// Unicode data copyright and license: see LICENSE.txt.\n\npackage unicode17\n\n'


def records(path):
    for line in path.read_text().splitlines():
        line = line.split('#', 1)[0].strip()
        if line:
            yield [part.strip() for part in line.split(';')]


def span(text):
    values = text.split('..')
    return int(values[0], 16), int(values[-1], 16)


def merge(ranges):
    result = []
    for lo, hi in sorted(ranges):
        if result and lo <= result[-1][1] + 1:
            result[-1] = (result[-1][0], max(hi, result[-1][1]))
        else:
            result.append((lo, hi))
    return result


def subtract(ranges, removed):
    result = []
    removed = merge(removed)
    index = 0
    for lo, hi in merge(ranges):
        while index < len(removed) and removed[index][1] < lo:
            index += 1
        cursor, part = lo, index
        while part < len(removed) and removed[part][0] <= hi:
            a, b = removed[part]
            if cursor < a:
                result.append((cursor, a - 1))
            cursor = max(cursor, b + 1)
            part += 1
        if cursor <= hi:
            result.append((cursor, hi))
    return result


def collapse(rows):
    result = []
    for lo, hi, value in sorted(rows):
        if result and lo == result[-1][1] + 1 and value == result[-1][2]:
            result[-1] = (result[-1][0], hi, value)
        else:
            result.append((lo, hi, value))
    return result


def quoted(text):
    return json.dumps(text, ensure_ascii=False)


def emit(path, text):
    path.write_text(HEADER + text)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--input', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    manifest = json.loads((ROOT / 'plans/golang-zero-dependencies/unicode-inputs.json').read_text())
    for item in manifest['inputs']:
        data = (args.input / item['url'].split('/17.0.0/', 1)[1]).read_bytes()
        if len(data) != item['bytes'] or hashlib.sha256(data).hexdigest() != item['sha256']:
            raise SystemExit(f'Input hash mismatch: {item["url"]}')
    args.output.mkdir(parents=True, exist_ok=True)
    ucd = args.input / 'ucd'
    categories, bidi, classes, decompositions, mirrored = {}, [], [], {}, []
    pending = None
    for fields in records(ucd / 'UnicodeData.txt'):
        point, name = int(fields[0], 16), fields[1]
        if name.endswith(', First>'):
            pending = point
            continue
        start = pending if name.endswith(', Last>') else point
        pending = None
        categories.setdefault(fields[2], []).append((start, point))
        bidi.append((start, point, fields[4]))
        if int(fields[3]):
            classes.append((start, point, int(fields[3])))
        if fields[5] and not fields[5].startswith('<'):
            decompositions[point] = [int(value, 16) for value in fields[5].split()]
        if fields[9] == 'Y':
            mirrored.append((start, point))
    assigned = merge([row for group in categories.values() for row in group])
    categories['Cn'] = subtract([(0, 0x10ffff)], assigned)
    for initial in 'CLMNPSZ':
        categories[initial] = merge([row for key, group in list(categories.items()) if len(key) == 2 and key.startswith(initial) for row in group])
    categories['LC'] = merge(categories['Lu'] + categories['Ll'] + categories['Lt'])
    properties = {'gc=' + name: merge(group) for name, group in categories.items()}
    properties.update({'ASCII': [(0, 127)], 'Any': [(0, 0x10ffff)], 'Assigned': assigned, 'Bidi_Mirrored': merge(mirrored)})
    for name in ['PropList.txt', 'DerivedCoreProperties.txt', 'DerivedNormalizationProps.txt', 'emoji/emoji-data.txt']:
        for fields in records(ucd / name):
            if fields[1] in BINARY:
                properties.setdefault(fields[1], []).append(span(fields[0]))
    scripts = {}
    for fields in records(ucd / 'Scripts.txt'):
        scripts.setdefault(fields[1], []).append(span(fields[0]))
    scripts['Unknown'] = subtract([(0, 0x10ffff)], [r for group in scripts.values() for r in group])
    script_names, category_names = {}, {}
    for fields in records(ucd / 'PropertyValueAliases.txt'):
        if fields[0] == 'sc':
            for alias in fields[1:]:
                script_names[alias] = fields[2]
        if fields[0] == 'gc':
            for alias in fields[1:]:
                category_names[alias] = fields[1]
    overrides, extensions = [], {}
    for fields in records(ucd / 'ScriptExtensions.txt'):
        pair = span(fields[0]); overrides.append(pair)
        for short in fields[1].split():
            extensions.setdefault(script_names[short], []).append(pair)
    # Hrkt is an aggregate alias in PropertyValueAliases, not a value assigned
    # by Script or Script_Extensions. ECMAScript does not accept it.
    valid_scripts = set(scripts) | set(extensions)
    script_names = {alias: name for alias, name in script_names.items() if name in valid_scripts}
    for name in valid_scripts:
        properties['sc=' + name] = merge(scripts.get(name, []))
        properties['scx=' + name] = merge(subtract(scripts.get(name, []), overrides) + extensions.get(name, []))
    aliases = {name: name for name in BINARY}
    for fields in records(ucd / 'PropertyAliases.txt'):
        if len(fields) > 1 and fields[1] in BINARY:
            for name in fields:
                aliases[name] = fields[1]
    missing = set(BINARY) - properties.keys()
    if missing:
        raise SystemExit(f'Missing required ECMAScript properties: {sorted(missing)}')
    alias_rows = {name: canonical for name, canonical in aliases.items()}
    alias_rows.update({'gc=' + alias: 'gc=' + name for alias, name in category_names.items()})
    for prefix in ['sc=', 'scx=']:
        alias_rows.update({prefix + alias: prefix + name for alias, name in script_names.items()})
    pool, entries, ids = [], [], {}
    for name, ranges in sorted(properties.items()):
        ranges = merge(ranges); ids[name] = len(entries)
        entries.append((len(pool), len(ranges))); pool += ranges
    text = 'var propertyRanges = [...]Range{\n' + ''.join(f'{{0x{lo:x},0x{hi:x}}},\n' for lo, hi in pool) + '}\n'
    text += 'var propertyEntries = [...]propertyEntry{\n' + ''.join(f'{{{offset},{count}}},\n' for offset, count in entries) + '}\n'
    text += 'var propertyAliases = [...]propertyAlias{\n' + ''.join(f'{{{quoted(alias)},{ids[name]}}},\n' for alias, name in sorted(alias_rows.items())) + '}\n'
    property_range_count = len(pool)
    emit(args.output / 'properties_data.go', text)
    excluded = []
    for fields in records(ucd / 'DerivedNormalizationProps.txt'):
        if fields[1] == 'Full_Composition_Exclusion':
            lo, hi = span(fields[0]); excluded += range(lo, hi + 1)
    excluded = set(excluded)
    compositions = sorted(((values[0] << 21) | values[1], point) for point, values in decompositions.items() if len(values) == 2 and point not in excluded)
    pool, entries = [], []
    for point, values in sorted(decompositions.items()):
        entries.append((point, len(pool), len(values))); pool += values
    text = 'var canonicalClasses = [...]classEntry{\n' + ''.join(f'{{0x{lo:x},0x{hi:x},{value}}},\n' for lo, hi, value in collapse(classes)) + '}\n'
    unstable = merge([span(f[0]) for f in records(ucd / 'DerivedNormalizationProps.txt') if f[1] == 'NFC_QC' and f[2] in ['N', 'M']])
    text += 'var nfcNonYes = [...]normalizationRange{\n' + ''.join(f'{{0x{lo:x},0x{hi:x}}},\n' for lo, hi in unstable) + '}\n'
    text += 'var decompositions = [...]decompositionEntry{\n' + ''.join(f'{{0x{point:x},{offset},{count}}},\n' for point, offset, count in entries) + '}\n'
    text += 'var decompositionRunes = [...]rune{' + ','.join(f'0x{point:x}' for point in pool) + '}\n'
    text += 'var compositions = [...]compositionEntry{\n' + ''.join(f'{{0x{key:x},0x{point:x}}},\n' for key, point in compositions) + '}\n'
    emit(args.output / 'normalization_data.go', text)
    folds = [(int(f[0], 16), int(f[2], 16)) for f in records(ucd / 'CaseFolding.txt') if f[1] in ['C', 'S']]
    emit(args.output / 'fold_data.go', 'var caseFolds = [...]foldEntry{\n' + ''.join(f'{{0x{a:x},0x{b:x}}},\n' for a, b in folds) + '}\n')
    statuses = {'valid': 'IDNAValid', 'ignored': 'IDNAIgnored', 'mapped': 'IDNAMapped', 'deviation': 'IDNADeviation', 'disallowed': 'IDNADisallowed', 'disallowed_STD3_valid': 'IDNASTD3Valid', 'disallowed_STD3_mapped': 'IDNASTD3Mapped'}
    text = 'var idnaMappings = [...]idnaEntry{\n'
    for fields in records(args.input / 'idna/IdnaMappingTable.txt'):
        lo, hi = span(fields[0]); mapping = ''.join(chr(int(c, 16)) for c in fields[2].split()) if len(fields) > 2 else ''
        text += f'{{0x{lo:x},0x{hi:x},{statuses[fields[1]]},{quoted(mapping)}}},\n'
    text += '}\nvar joiningTypes = [...]joiningEntry{\n' + ''.join(f'{{0x{span(f[0])[0]:x},0x{span(f[0])[1]:x},{quoted(f[1])}}},\n' for f in sorted(records(ucd / 'extracted/DerivedJoiningType.txt'), key=lambda f: span(f[0])[0])) + '}\n'
    text += 'var bidiClasses = [...]bidiEntry{\n' + ''.join(f'{{0x{lo:x},0x{hi:x},{quoted(value)}}},\n' for lo, hi, value in collapse(bidi)) + '}\n'
    text += 'var idnaMarks = [...]idnaMarkEntry{\n' + ''.join(f'{{0x{lo:x},0x{hi:x}}},\n' for lo, hi in categories['M']) + '}\n'
    emit(args.output / 'idna_data.go', text)
    print(json.dumps({'unicode': manifest['unicode'], 'properties': len(properties), 'propertyRanges': property_range_count, 'verifiedInputs': len(manifest['inputs'])}))


if __name__ == '__main__':
    main()
