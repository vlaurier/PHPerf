<?php
/**
 * PHPerf — écriture du profil en fin de requête HTTP.
 *
 * Inclus automatiquement par phperf-prepend.php (cas nominal) ; peut aussi
 * être déclaré seul via auto_append_file si l'amorce est gérée autrement.
 * Sans $GLOBALS['PHPPERF_ACTIVE'], il n'a aucun effet.
 *
 * La normalisation du dump (dupliquée volontairement avec
 * phperf-profile.php pour rester un fichier autonome copiable tel quel)
 * garantit le contrat attendu par le parseur Go : entiers, champs
 * ct/wt/cpu/mu/pmu toujours présents, racine "main()" définie.
 */

declare(strict_types=1);

register_shutdown_function(static function (): void {
    if (empty($GLOBALS['PHPPERF_ACTIVE']) || !function_exists('xhprof_disable')) {
        return;
    }

    $data = xhprof_disable();

    $maxWt = 0;
    foreach ($data as $key => $row) {
        $data[$key] = [
            'ct' => (int) ($row['ct'] ?? 0),
            'wt' => (int) ($row['wt'] ?? 0),
            'cpu' => (int) ($row['cpu'] ?? 0),
            'mu' => (int) ($row['mu'] ?? 0),
            'pmu' => (int) ($row['pmu'] ?? 0),
        ];
        if ($key !== 'main()') {
            $maxWt = max($maxWt, $data[$key]['wt']);
        }
    }
    if (!isset($data['main()'])) {
        $data = ['main()' => ['ct' => 1, 'wt' => $maxWt, 'cpu' => $maxWt,
                              'mu' => 0, 'pmu' => 0]] + $data;
    }

    $json = json_encode($data, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
    if ($json === false) {
        error_log('[phperf] encodage JSON impossible : ' . json_last_error_msg());
        return;
    }

    $dir = rtrim(getenv('PHPERF_OUTPUT_DIR') ?: sys_get_temp_dir(), '/');
    $file = $dir . '/phperf-' . date('Ymd-His') . '-' . bin2hex(random_bytes(3)) . '.json';

    if (@file_put_contents($file, $json . "\n") === false) {
        error_log("[phperf] écriture du profil impossible dans $file");
        return;
    }
    error_log("[phperf] profil écrit : $file (" . count($data) . " entrées)");
});
