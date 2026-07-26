import assert from 'node:assert/strict';
import test from 'node:test';

import { workspaceLabel } from './workspace-picker.js';

test('workspace label always uses the full selected path', function () {
    assert.equal(
        workspaceLabel('/media/jahrulnr/storage/workspace/BuatPostingan', '/workspace'),
        '/media/jahrulnr/storage/workspace/BuatPostingan'
    );
});

test('workspace label falls back to the backend current directory', function () {
    assert.equal(workspaceLabel('', '/media/jahrulnr/storage/workspace/BuatPostingan'), '/media/jahrulnr/storage/workspace/BuatPostingan');
    assert.equal(workspaceLabel('', ''), 'Workspace');
});
