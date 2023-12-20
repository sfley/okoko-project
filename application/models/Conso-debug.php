<?php

class Conso extends CI_Model {

    function get_conso($year) {
        $result_stock_annuel = array();
        $result_stock_mois = array();

        for ($month = 3; $month <= 12; $month++) {
            $monthFormatted = str_pad($month, 2, '0', STR_PAD_LEFT);
            $value_select_stock_month_Date = "{$year}-{$monthFormatted}-01";

            // Find the first date entry
            $query_first_date = $this->db->query(
                "SELECT MIN(DATE) AS first_date
                 FROM `json_import` 
                 WHERE DATE >= ? AND DATE <= LAST_DAY(?)
                 AND heure < '00:01:00';",
                array($value_select_stock_month_Date, $value_select_stock_month_Date)
            );

            // Find the last date entry
            $query_last_date = $this->db->query(
                "SELECT MAX(DATE) AS last_date
                 FROM `json_import` 
                 WHERE DATE >= ? AND DATE <= LAST_DAY(?)
                 AND heure < '00:01:00';",
                array($value_select_stock_month_Date, $value_select_stock_month_Date)
            );

            // Get PE1_Fuellstand_kg for the first and last date entries
            $first_date_result = $query_first_date->row();
            $last_date_result = $query_last_date->row();

            if (!$first_date_result || !$last_date_result) {
                // Handle error or set default values
                continue;
            }

            $first_date = $first_date_result->first_date;
            $last_date = $last_date_result->last_date;

            $query_first_value = $this->db->query(
                "SELECT PE1_Fuellstand_kg AS first_value
                 FROM `json_import` 
                 WHERE DATE = ? AND heure < '00:01:00';",
                array($first_date)
            );

            $query_last_value = $this->db->query(
                "SELECT PE1_Fuellstand_kg AS last_value
                 FROM `json_import` 
                 WHERE DATE = ? AND heure < '00:01:00';",
                array($last_date)
            );

            // Get recharge value
            $query_recharge = $this->db->query(
                "SELECT COALESCE(SUM(CASE WHEN recharge IS NOT NULL THEN recharge END), 0) AS total_recharge
                 FROM `json_import` t
                 WHERE t.DATE >= ? AND t.DATE <= DATE_ADD(?, INTERVAL 7 DAY) AND t.heure < '00:01:00';",
                array($value_select_stock_month_Date, $value_select_stock_month_Date)
            );

            // Fetch results
            $first_value_result = $query_first_value->row();
            $last_value_result = $query_last_value->row();
            $total_recharge_result = $query_recharge->row();

            if (!$first_value_result || !$last_value_result || !$total_recharge_result) {
                // Handle error or set default values
                continue;
            }

            $first_value = $first_value_result->first_value;
            $last_value = $last_value_result->last_value;
            $total_recharge = $total_recharge_result->total_recharge;

            // Ensure month names are enclosed in single quotes
            $monthName = "'" . date('F', mktime(0, 0, 0, $month, 1)) . "'";

            // Calculate conso
            $conso = ABS($first_value + $total_recharge) - $last_value;

            echo $monthName;
            echo "first_value:"; var_dump($first_value);
            echo "last_value:"; var_dump($last_value);
            echo "reload:"; var_dump($total_recharge);
            echo "conso:"; var_dump($conso);
            echo "<br>";

            $result_stock_mois[] = array(
                'mois' => $monthName,
                'conso' => $conso ?? 0 // Use 0 if the value is still null
            );
        }

        $result_stock_annuel['mois'] = implode(array_column($result_stock_mois, 'mois'), ",");
        $result_stock_annuel['conso'] = implode(array_column($result_stock_mois, 'conso'), ",");
        $result_stock_annuel['name'] = 'consommation ' . $year;
        $result_stock_annuel['annee'] = $year;

        // var_dump($result_stock_annuel); // Dumping the result for debugging

        return $result_stock_annuel;
    }

 



    function get_conso_day() {
        $result_stock_day = array();

        $query_stock_day = $this->db->query('SELECT Date, PE1_Fuellstand_kg as conso_day FROM `json_import` WHERE heure < "00:00:40" ORDER BY date');

        if ($query_stock_day->num_rows() > 0) {
            $stock = $query_stock_day->result_array();
            foreach ($stock as $row) {
                $result_stock_day_date[] = "'" . $row['Date'] . "'";
            }
            foreach ($stock as $row) {
                $result_stock_day_data[] = $row['conso_day'];
            }

            $result_stock_day['date'] = implode($result_stock_day_date, ",");
            $result_stock_day['data'] = implode($result_stock_day_data, ",");

            // var_dump($result_stock_day); // Dumping the result for debugging

            return $result_stock_day;
        }
    }

    function get_conso_now() {
        $result_stock_now = array();

        $query_stock_now = $this->db->query('SELECT PE1_Fuellstand_kg as conso_now FROM `json_import` ORDER BY date DESC LIMIT 1');

        if ($query_stock_now->num_rows() > 0) {
            $stock = $query_stock_now->result_array();
            foreach ($stock as $row) {
                $result_stock_day_data[] = $row['conso_now'];
            }

            $result_stock_now['data'] = implode($result_stock_day_data, ",");

            // var_dump($result_stock_now); // Dumping the result for debugging

            return $result_stock_now;
        }
    }
}
