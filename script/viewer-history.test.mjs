import assert from 'node:assert/strict';
import test from 'node:test';
import {pingHistoryHours} from '../komari-next/src/lib/viewerHistory.ts';

test('short and realtime shares never request a wider history window',()=>{
  assert.equal(pingHistoryHours(null),0);
  assert.equal(pingHistoryHours({is_share_view:true,ping_record_preserve_time:1}),1);
  assert.equal(pingHistoryHours({is_share_view:true,ping_record_preserve_time:0}),0);
  assert.equal(pingHistoryHours({is_share_view:true}),0);
  assert.equal(pingHistoryHours({is_share_view:true,ping_record_preserve_time:NaN}),0);
  assert.equal(pingHistoryHours({is_share_view:true,ping_record_preserve_time:24},12),12);
  assert.equal(pingHistoryHours({is_share_view:true,ping_record_preserve_time:168}),24);
});
test('management preserves its requested history independently of viewer policy',()=>{
  assert.equal(pingHistoryHours({ping_record_preserve_time:1}),24);
  assert.equal(pingHistoryHours({is_share_view:false,ping_record_preserve_time:0},12),12);
  assert.equal(pingHistoryHours({},0),0);
});
