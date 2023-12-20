<?php
Class Temp extends CI_Model {
	
function get_tempHK1() {

$q = $this->db->query('select HK1_RTIst_Temp_Ambiante as HK1_Temp_Ambiante,CONCAT(DATE_FORMAT(Date,"%d/%m")," ",DATE_FORMAT(Heure,"%H:%i"))
as Date from json_import where date between now() - INTERVAL 7 DAY and now()  AND MINUTE(heure) IN (0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55) LIMIT 2000 ');



if($q->num_rows() > 0) {
foreach ($q->result() as $row)
{
$data[] = $row;
}
return $data;
}
	
	
}	

function get_tempHK2() {

$q = $this->db->query('select HK2_RTIst_Temp_Ambiante as HK2_Temp_Ambiante,CONCAT(DATE_FORMAT(Date,"%d/%m")," ",DATE_FORMAT(Heure,"%H:%i"))
as Date from json_import where date between now() - INTERVAL 7 DAY and now()  AND MINUTE(heure) IN (0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55) LIMIT 2000 ');

if($q->num_rows() > 0) {
foreach ($q->result() as $row)
{
$data[] = $row;
}
return $data;
}
	
	
}	

function get_tempWW1() {

$q = $this->db->query('select WW1_EinTIst_Temp_ECS as WW1_Temp_ECS from json_import where date between now() - INTERVAL 7 DAY and now()  
AND MINUTE(heure) IN (0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55) LIMIT 2000 ');

if($q->num_rows() > 0) {
foreach ($q->result() as $row)
{
$data[] = $row;
}
return $data;
}
	
	
}	

function get_tempEXT() {

$q = $this->db->query('select AT_Temp_Ext as Temp_EXT from json_import where date between now() - INTERVAL 7 DAY and now()  AND MINUTE(heure) IN (0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55) LIMIT 2000 ');

if($q->num_rows() > 0) {
foreach ($q->result() as $row)
{
$data[] = $row;
}
return $data;
}
	
	
}
	
}

