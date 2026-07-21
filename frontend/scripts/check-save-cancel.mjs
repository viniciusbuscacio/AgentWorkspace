import fs from 'node:fs/promises';
import path from 'node:path';
import ts from 'typescript';

const rootDir = path.resolve(import.meta.dirname, '..');
const srcDir = path.join(rootDir, 'src');
const saveCancelPath = path.join(srcDir, 'components', 'patterns', 'SaveCancelActions.tsx');
const fixedWidthExport = "export const SAVE_CANCEL_BUTTON_CLASS = 'w-24';";

const violations = [];

async function listTsxFiles(dir) {
  const entries = await fs.readdir(dir, { withFileTypes: true });
  const files = [];
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...await listTsxFiles(fullPath));
      continue;
    }
    if (entry.isFile() && entry.name.endsWith('.tsx') && !entry.name.endsWith('.test.tsx')) {
      files.push(fullPath);
    }
  }
  return files;
}

function jsxTagName(tagName, sourceFile) {
  return tagName.getText(sourceFile);
}

function visibleText(children) {
  let text = '';
  for (const child of children) {
    if (ts.isJsxText(child)) {
      text += child.getText();
      continue;
    }
    if (ts.isJsxExpression(child) && child.expression && ts.isStringLiteral(child.expression)) {
      text += child.expression.text;
    }
  }
  return text.trim();
}

function hasJsxAttribute(attributes, name) {
  return attributes.properties.some((prop) => (
    ts.isJsxAttribute(prop) && prop.name.getText() === name && prop.initializer !== undefined
  ));
}

function assertSaveCancelProps(attributes, sourceFile, filePath, label) {
  const hasSave = hasJsxAttribute(attributes, 'onSave');
  const hasCancel = hasJsxAttribute(attributes, 'onCancel');
  if (hasSave && hasCancel) return;
  const line = sourceFile.getLineAndCharacterOfPosition(attributes.pos).line + 1;
  const missing = [
    hasSave ? '' : 'onSave',
    hasCancel ? '' : 'onCancel',
  ].filter(Boolean).join(' and ');
  violations.push(`${path.relative(rootDir, filePath)}:${line}: <${label}> must include both onSave and onCancel; missing ${missing}.`);
}

function inspectJsx(node, sourceFile, filePath) {
  if (ts.isJsxElement(node)) {
    const tag = jsxTagName(node.openingElement.tagName, sourceFile);
    const label = visibleText(node.children);
    const isRawSaveCancelButton = (tag === 'Button' || tag === 'button') && (label === 'Save' || label === 'Cancel' || label.startsWith('Saving'));
    if (isRawSaveCancelButton && filePath !== saveCancelPath) {
      violations.push(`${path.relative(rootDir, filePath)}: raw <${tag}>${label}</${tag}> is not allowed; use <SaveCancelActions onSave onCancel>.`);
    }
    if (tag === 'SaveCancelActions') {
      assertSaveCancelProps(node.openingElement.attributes, sourceFile, filePath, tag);
    }
  }
  if (ts.isJsxSelfClosingElement(node)) {
    const tag = jsxTagName(node.tagName, sourceFile);
    if (tag === 'SaveCancelActions') {
      assertSaveCancelProps(node.attributes, sourceFile, filePath, tag);
    }
  }
  ts.forEachChild(node, (child) => inspectJsx(child, sourceFile, filePath));
}

function objectLiteralHasProperty(objectLiteral, name) {
  return objectLiteral.properties.some((prop) => (
    ts.isPropertyAssignment(prop) && prop.name.getText().replace(/^['"]|['"]$/g, '') === name
  ) || (
    ts.isShorthandPropertyAssignment(prop) && prop.name.getText() === name
  ));
}

function inspectCalls(node, sourceFile, filePath) {
  if (
    ts.isCallExpression(node) &&
    node.expression.getText(sourceFile) === 'saveCancelToolbarActions'
  ) {
    const [firstArg] = node.arguments;
    if (firstArg && ts.isObjectLiteralExpression(firstArg)) {
      const hasSave = objectLiteralHasProperty(firstArg, 'onSave');
      const hasCancel = objectLiteralHasProperty(firstArg, 'onCancel');
      if (!hasSave || !hasCancel) {
        const line = sourceFile.getLineAndCharacterOfPosition(firstArg.pos).line + 1;
        const missing = [
          hasSave ? '' : 'onSave',
          hasCancel ? '' : 'onCancel',
        ].filter(Boolean).join(' and ');
        violations.push(`${path.relative(rootDir, filePath)}:${line}: saveCancelToolbarActions(...) must include both onSave and onCancel; missing ${missing}.`);
      }
    }
  }
  ts.forEachChild(node, (child) => inspectCalls(child, sourceFile, filePath));
}

async function checkSaveCancelComponent() {
  const source = await fs.readFile(saveCancelPath, 'utf8');
  const fixedWidthUses = source.match(/className=\{SAVE_CANCEL_BUTTON_CLASS\}/g)?.length ?? 0;
  if (!source.includes(fixedWidthExport)) {
    violations.push('src/components/patterns/SaveCancelActions.tsx: SAVE_CANCEL_BUTTON_CLASS must stay fixed at w-24.');
  }
  if (fixedWidthUses !== 2) {
    violations.push('src/components/patterns/SaveCancelActions.tsx: Save and Cancel buttons must both use SAVE_CANCEL_BUTTON_CLASS.');
  }
}

await checkSaveCancelComponent();

for (const filePath of await listTsxFiles(srcDir)) {
  const source = await fs.readFile(filePath, 'utf8');
  const sourceFile = ts.createSourceFile(filePath, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  inspectJsx(sourceFile, sourceFile, filePath);
  inspectCalls(sourceFile, sourceFile, filePath);
}

if (violations.length > 0) {
  console.error('Save/Cancel contract failed:\n');
  for (const violation of violations) {
    console.error(`- ${violation}`);
  }
  process.exit(1);
}
